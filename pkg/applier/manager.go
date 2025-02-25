/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package applier

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/fsnotify/fsnotify"
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.log.WithError(err).Error("Failed to create watcher")
		return
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			m.log.WithError(err).Error("Failed to close watcher")
		}
	}()

	err = watcher.Add(m.bundleDir)
	if err != nil {
		m.log.WithError(err).Error("Failed to watch bundle directory")
		return
	}

	m.log.Info("Starting watch loop")

	// Add all directories after the bundle dir has been added to the watcher.
	// Doing it the other way round introduces a race condition when directories
	// get created after the initial listing but before the watch starts.

	dirs, err := dir.GetAll(m.bundleDir)
	if err != nil {
		m.log.WithError(err).Error("Failed to read bundle directory")
		return
	}

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil) // satisfy linter, not required for correctness
	stacks := make(map[string]stack, len(dirs))

	for _, dir := range dirs {
		m.createStack(ctx, stacks, path.Join(m.bundleDir, dir))
	}

	for {
		select {
		case err := <-watcher.Errors:
			m.log.WithError(err).Error("Watch error")
			cancel(err)

		case event := <-watcher.Events:
			switch event.Op {
			case fsnotify.Create:
				if dir.IsDirectory(event.Name) {
					m.createStack(ctx, stacks, event.Name)
				}
			case fsnotify.Remove:
				m.removeStack(ctx, stacks, event.Name)
			}

		case <-ctx.Done():
			m.log.Infof("Watch loop done (%v)", context.Cause(ctx))
			for _, stack := range stacks {
				<-stack.stopped
			}

			return
		}
	}
}

func (m *Manager) createStack(ctx context.Context, stacks map[string]stack, name string) {
	// safeguard in case the fswatcher would trigger an event for an already existing stack
	if _, ok := stacks[name]; ok {
		return
	}

	log := m.log.WithField("stack", name)

	stackName := filepath.Base(name)
	if slices.Contains(m.IgnoredStacks, stackName) {
		if err := file.AtomicWithTarget(filepath.Join(name, "ignored.txt")).WriteString(
			"The " + stackName + " stack is handled internally.\n" +
				"This directory is ignored and can be safely removed.\n",
		); err != nil {
			log.WithError(err).Warn("Failed to write ignore notice")
		}
		return
	}

	ctx, cancel := context.WithCancelCause(ctx)
	stopped := make(chan struct{})

	stack := stack{cancel, stopped, NewStackApplier(name, m.KubeClientFactory)}
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
