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

	oswatch "github.com/k0sproject/k0s/internal/os/watch"
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
	stacks := make(map[string]stack)
	stackCtx, cancel := context.WithCancelCause(ctx)

	defer func() {
		cancel(context.Cause(ctx))
		for _, stack := range stacks {
			<-stack.stopped
		}
	}()

	err := oswatch.Dir(stackCtx, m.bundleDir, oswatch.HandlerFunc(func(e oswatch.Event) {
		switch e := e.(type) {
		case *oswatch.Established:
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
					m.createStack(stackCtx, stacks, entry.Name())
				}
			}

		case *oswatch.Touched:
			if info, err := e.Info(); err == nil {
				if info.IsDir() {
					m.createStack(stackCtx, stacks, e.Name)
				}
			} else if errors.Is(err, os.ErrNotExist) {
				m.removeStack(stackCtx, stacks, e.Name)
			} else {
				cancel(err)
			}

		case *oswatch.Gone:
			m.removeStack(stackCtx, stacks, e.Name)
		}
	}))

	if err != nil {
		cancel(err)
	} else if ctx.Err() == nil {
		err = context.Cause(stackCtx)
	}

	if err != nil {
		m.log.WithError(err).Error("Failed to watch manifests directory")
	} else {
		m.log.Infof("Watch loop done (%v)", context.Cause(ctx))
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

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			log.Info("Running stack")
			if err := stack.Run(ctx); err != nil {
				log.WithError(err).Error("Failed to run stack")
			}
		}, 1*time.Minute)

		log.Infof("Stack done (%v)", context.Cause(ctx))
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
