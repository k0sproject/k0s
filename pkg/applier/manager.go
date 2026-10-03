// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	fswatch "github.com/k0sproject/k0s/internal/os/fs/watch"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/sirupsen/logrus"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	K0sVars           *config.CfgVars
	IgnoredStacks     []string
	KubeClientFactory kubeutil.ClientFactoryInterface

	bundleDir string
	stop      func()
	log       *logrus.Entry

	LeaderElector leaderelector.Interface
}

var _ manager.Component = (*Manager)(nil)

type stack = struct {
	cancel  context.CancelCauseFunc
	stopped <-chan struct{}
	*StackApplier
}

// Init initializes the Manager
func (m *Manager) Init(ctx context.Context) error {
	err := dir.Init(m.K0sVars.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return fmt.Errorf("failed to create manifest bundle dir %s: %w", m.K0sVars.ManifestsDir, err)
	}
	m.log = logrus.WithField("component", constant.ApplierManagerComponentName)
	m.bundleDir = m.K0sVars.ManifestsDir

	return nil
}

// Run runs the Manager
func (m *Manager) Start(context.Context) error {
	ctx, cancel := context.WithCancelCause(context.Background())
	stopped := make(chan struct{})

	m.stop = func() {
		cancel(errors.New("applier manager is stopping"))
		<-stopped
	}

	go func() {
		defer close(stopped)
		leaderelection.RunLeaderTasks(ctx, m.LeaderElector.CurrentStatus, func(ctx context.Context) {
			wait.UntilWithContext(ctx, m.runWatchers, time.Minute)
		})
	}()

	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	if m.stop != nil {
		m.stop()
	}
	return nil
}

func (m *Manager) runWatchers(ctx context.Context) {
	outerCtx := ctx
	stacks := make(map[string]stack)
	ctx, cancel := context.WithCancelCause(outerCtx)

	defer func() {
		cancel(nil)
		for _, stack := range stacks {
			<-stack.stopped
		}
	}()

	err := fswatch.Dir(ctx, m.bundleDir, fswatch.HandlerFunc(func(e fswatch.Event) {
		switch e := e.(type) {
		case *fswatch.Established:
			// Add all directories after activating the watch. Doing so before
			// starting the watch introduces a race condition if directories are
			// created after the initial listing but before the watch starts.

			entries, err := os.ReadDir(e.Path)
			if err != nil {
				cancel(err)
				return
			}

			for _, entry := range entries {
				if entry.IsDir() {
					m.createStack(ctx, stacks, entry.Name())
				}
			}

		case *fswatch.Desynced:
			// Re-read all directories from disk.
			entries, err := os.ReadDir(e.Path)
			if err != nil {
				cancel(err)
				return
			}

			// Create the stacks and record their names.
			var seenStacks []string
			for _, entry := range entries {
				if entry.IsDir() {
					name := entry.Name()
					m.createStack(ctx, stacks, name)
					seenStacks = append(seenStacks, name)
				}
			}

			// Remove all stacks whose directory has gone.
			for name := range stacks {
				if !slices.Contains(seenStacks, name) {
					m.removeStack(ctx, stacks, name)
				}
			}

		case *fswatch.Changed:
			if info, err := e.Stat(); err == nil {
				if info.IsDir() {
					m.createStack(ctx, stacks, e.Name)
				}
			} else if errors.Is(err, os.ErrNotExist) {
				m.removeStack(ctx, stacks, e.Name)
			} else if stack, ok := stacks[e.Name]; ok {
				// We have a stack with that name, but have trouble getting the
				// directory info: Stop the stack, but don't attempt to remove
				// it from the cluster.
				m.log.WithField("stack", e.Name).WithError(err).
					Error("Failed to get directory info, stopping to watch stack")
				stack.cancel(fmt.Errorf("failed to get stack directory info: %w", err))
				delete(stacks, e.Name)
				<-stack.stopped
			} else {
				m.log.WithError(err).Warn("Failed to get path info while watching manifest directory")
			}
		}
	}))

	if err != nil {
		cancel(err)
	} else if outerCtx.Err() == nil {
		err = context.Cause(ctx)
	}

	if err != nil {
		m.log.WithError(err).WithField("path", m.bundleDir).Error("Failed to watch manifests directory")
	} else {
		m.log.Infof("Watch loop done (%v)", context.Cause(outerCtx))
	}
}

func (m *Manager) createStack(ctx context.Context, stacks map[string]stack, name string) {
	// safeguard in case the fswatcher would trigger an event for an already existing stack
	if _, ok := stacks[name]; ok {
		return
	}

	log := m.log.WithField("stack", name)

	if slices.Contains(m.IgnoredStacks, name) {
		if err := file.AtomicWithTarget(filepath.Join(m.bundleDir, name, "ignored.txt")).WriteString(
			"The " + name + " stack is handled internally.\n" +
				"This directory is ignored and can be safely removed.\n",
		); err != nil {
			log.WithError(err).Warn("Failed to write ignore notice")
		}
		return
	}

	ctx, cancel := context.WithCancelCause(ctx)
	stopped := make(chan struct{})

	stack := stack{cancel, stopped, NewStackApplier(filepath.Join(m.bundleDir, name), m.KubeClientFactory)}
	stacks[name] = stack

	go func() {
		defer close(stopped)

		for {
			log.Info("Running stack")
			if err := stack.Run(ctx); err != nil {
				gone := errors.Is(err, fswatch.ErrWatchedDirectoryGone)
				if gone {
					log.WithError(err).Info("Stack directory is gone, awaiting deletion")
				} else {
					log.WithError(err).Error("Failed to run stack")
				}

				select {
				case <-time.After(wait.Jitter(1*time.Minute, 0.3)):
					if gone {
						log.Error("Stack directory gone, but stack wasn't deleted, retrying")
					}
					continue
				case <-ctx.Done():
				}
			}

			log.Infof("Stack done (%v)", context.Cause(ctx))
			break
		}
	}()
}

func (m *Manager) removeStack(ctx context.Context, stacks map[string]stack, name string) {
	stack, ok := stacks[name]
	if !ok {
		m.log.
			WithField("path", name).
			Debug("attempted to remove non-existent stack, probably not a directory")
		return
	}

	delete(stacks, name)
	stack.cancel(errors.New("stack removed"))
	<-stack.stopped

	log := m.log.WithField("stack", name)
	if err := stack.DeleteStack(ctx); err != nil {
		log.WithError(err).Error("Failed to delete stack")
		return
	}

	log.Info("Stack deleted successfully")
}
