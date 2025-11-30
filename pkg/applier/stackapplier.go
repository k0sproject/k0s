// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// StackApplier applies a stack whenever the files on disk change.
type StackApplier struct {
	log  logrus.FieldLogger
	path string

	doApply, doDelete func(context.Context) error
}

// NewStackApplier crates new stack applier to manage a stack
func NewStackApplier(path string, kubeClientFactory kubernetes.ClientFactoryInterface) *StackApplier {
	var mu sync.Mutex
	applier := NewApplier(path, kubeClientFactory)

	return &StackApplier{
		log:  logrus.WithField("component", "applier-"+applier.Name),
		path: path,

		doApply: func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			return applier.Apply(ctx)
		},

		doDelete: func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			return applier.Delete(ctx)
		},
	}
}

// Run watches the stack for updates and executes the initial apply.
func (s *StackApplier) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil // The context is already done.
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	trigger := make(chan struct{}, 1)
	watchErr := make(chan error, 1)
	go func() { watchErr <- s.runWatcher(watcher, trigger, ctx.Done()) }()

	if err := watcher.Add(s.path); err != nil {
		return fmt.Errorf("failed to watch %q: %w", s.path, err)
	}

	for {
		select {
		case <-trigger:
			s.apply(ctx)
		case err := <-watchErr:
			return err
		}
	}
}

func (s *StackApplier) runWatcher(watcher *fsnotify.Watcher, trigger chan<- struct{}, stop <-chan struct{}) (err error) {
	defer func() { err = errors.Join(err, watcher.Close()) }()

	const timeout = 1 * time.Second // debounce events for one second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Watch for changes in the path and any symlink targets
	if err := s.setupSymlinkWatching(watcher); err != nil {
		s.log.WithError(err).Warn("Failed to setup symlink watching")
	}

	for {
		select {
		case err := <-watcher.Errors:
			return fmt.Errorf("while watching stack: %w", err)

		case event := <-watcher.Events:
			// Only consider events on manifest files
			if match, _ := filepath.Match(manifestFilePattern, filepath.Base(event.Name)); !match {
				// Check if this is a symlink creation/removal that might affect manifest files
				if s.handleSymlinkEvent(watcher, event) {
					timer.Reset(timeout)
				}
				continue
			}
			timer.Reset(timeout)

		case <-timer.C:
			select {
			case trigger <- struct{}{}:
			default:
			}

		case <-stop:
			return nil
		}
	}
}

func (s *StackApplier) apply(ctx context.Context) {
	s.log.Info("Applying manifests")

	err := retry.Do(
		func() error { return s.doApply(ctx) },
		retry.OnRetry(func(attempt uint, err error) {
			s.log.WithError(err).Warnf("Failed to apply manifests in attempt #%d, retrying after backoff", attempt+1)
		}),
		retry.Context(ctx),
		retry.LastErrorOnly(true),
	)

	if err != nil {
		s.log.WithError(err).Error("Failed to apply manifests")
	}
}

// DeleteStack deletes the associated stack
func (s *StackApplier) DeleteStack(ctx context.Context) error {
	return s.doDelete(ctx)
}

// setupSymlinkWatching adds watches for symlink targets in the stack path
func (s *StackApplier) setupSymlinkWatching(watcher *fsnotify.Watcher) error {
	// Check if the stack path itself is a symlink
	if s.isPathSymlink(s.path) {
		resolvedPath, err := filepath.EvalSymlinks(s.path)
		if err != nil {
			return fmt.Errorf("failed to resolve stack path symlink: %w", err)
		}
		
		// Add watch for the resolved target
		if err := watcher.Add(resolvedPath); err != nil {
			return fmt.Errorf("failed to watch resolved path %q: %w", resolvedPath, err)
		}
		s.log.WithField("resolved_path", resolvedPath).Debug("Added watch for symlink target")
	}
	
	// Check for symlinks within the stack directory
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return fmt.Errorf("failed to read stack directory: %w", err)
	}
	
	for _, entry := range entries {
		fullPath := filepath.Join(s.path, entry.Name())
		if s.isPathSymlink(fullPath) {
			resolvedPath, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				s.log.WithError(err).WithField("symlink", fullPath).Warn("Failed to resolve symlink")
				continue
			}
			
			// Add watch for the resolved target
			if err := watcher.Add(resolvedPath); err != nil {
				s.log.WithError(err).WithField("resolved_path", resolvedPath).Warn("Failed to watch symlink target")
				continue
			}
			s.log.WithField("symlink", fullPath).WithField("resolved_path", resolvedPath).Debug("Added watch for symlink target")
		}
	}
	
	return nil
}

// handleSymlinkEvent handles filesystem events related to symlinks
func (s *StackApplier) handleSymlinkEvent(watcher *fsnotify.Watcher, event fsnotify.Event) bool {
	// Check if the event path is a symlink
	if !s.isPathSymlink(event.Name) {
		return false
	}
	
	switch event.Op {
	case fsnotify.Create:
		// Symlink created - add watch for its target
		resolvedPath, err := filepath.EvalSymlinks(event.Name)
		if err != nil {
			s.log.WithError(err).WithField("symlink", event.Name).Warn("Failed to resolve new symlink")
			return false
		}
		
		if err := watcher.Add(resolvedPath); err != nil {
			s.log.WithError(err).WithField("resolved_path", resolvedPath).Warn("Failed to watch new symlink target")
			return false
		}
		
		s.log.WithField("symlink", event.Name).WithField("resolved_path", resolvedPath).Debug("Added watch for new symlink target")
		return true
		
	case fsnotify.Remove:
		// Symlink removed - we don't need to do anything special as the watcher
		// will automatically stop watching the resolved path
		s.log.WithField("symlink", event.Name).Debug("Symlink removed")
		return true
	}
	
	return false
}

// isPathSymlink checks if the given path is a symlink
func (s *StackApplier) isPathSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
