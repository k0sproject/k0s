// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"context"
	"fmt"
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

// Run executes the initial apply and watches the stack for updates.
func (s *StackApplier) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil // The context is already done.
	}

	trigger := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	go func() {
		defer close(trigger)
		defer func() {
			if err := watcher.Close(); err != nil {
				s.log.WithError(err).Error("Failed to close watcher")
			}
		}()
		err = s.runWatcher(watcher, trigger, ctx.Done())
	}()

	if addErr := watcher.Add(s.path); addErr != nil {
		return fmt.Errorf("failed to watch %q: %w", s.path, addErr)
	}

	for range trigger {
		s.apply(ctx)
	}

	return err
}

func (s *StackApplier) runWatcher(watcher *fsnotify.Watcher, trigger chan<- struct{}, stop <-chan struct{}) error {
	const timeout = 1 * time.Second // debounce events for one second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case err := <-watcher.Errors:
			return fmt.Errorf("while watching stack: %w", err)

		case event := <-watcher.Events:
			// Only consider events on manifest files
			if match, _ := filepath.Match(manifestFilePattern, filepath.Base(event.Name)); !match {
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
