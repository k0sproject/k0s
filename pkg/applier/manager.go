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
	"fmt"
	"path"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	K0sVars           *config.CfgVars
	KubeClientFactory kubeutil.ClientFactoryInterface

	// client               kubernetes.Interface
	applier       Applier
	bundlePath    string
	cancelWatcher context.CancelFunc
	log           *logrus.Entry
	stacks        map[string]stack

	LeaderElector leaderelector.Interface
}

var _ manager.Component = (*Manager)(nil)

type stack = struct {
	context.CancelFunc
	*StackApplier
}

// Init initializes the Manager
func (m *Manager) Init(ctx context.Context) error {
	err := dir.Init(m.K0sVars.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return fmt.Errorf("failed to create manifest bundle dir %s: %w", m.K0sVars.ManifestsDir, err)
	}
	m.log = logrus.WithField("component", "applier-manager")
	m.stacks = make(map[string]stack)
	m.bundlePath = m.K0sVars.ManifestsDir

	m.applier = NewApplier(m.K0sVars.ManifestsDir, m.KubeClientFactory)

	m.LeaderElector.AddAcquiredLeaseCallback(func() {
		watcherCtx, cancel := context.WithCancel(ctx)
		m.cancelWatcher = cancel
		go func() {
			_ = m.runWatchers(watcherCtx)
		}()
	})
	m.LeaderElector.AddLostLeaseCallback(func() {
		if m.cancelWatcher != nil {
			m.cancelWatcher()
		}
	})

	return err
}

// Run runs the Manager
func (m *Manager) Start(_ context.Context) error {
	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	if m.cancelWatcher != nil {
		m.cancelWatcher()
	}
	return nil
}

func (m *Manager) runWatchers(ctx context.Context) error {
	log := logrus.WithField("component", "applier-manager")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithError(err).Error("failed to create watcher")
		return err
	}
	defer watcher.Close()

	err = watcher.Add(m.bundlePath)
	if err != nil {
		log.Warnf("Failed to start watcher: %s", err.Error())
	}

	// Add all directories after the bundle dir has been added to the watcher.
	// Doing it the other way round introduces a race condition when directories
	// get created after the initial listing but before the watch starts.

	dirs, err := dir.GetAll(m.bundlePath)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		m.createStack(ctx, path.Join(m.bundlePath, dir))
	}

	for {
		select {
		case err, ok := <-watcher.Errors:
			if !ok {
				return err
			}

			log.Warnf("watch error: %s", err.Error())
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			switch event.Op {
			case fsnotify.Create:
				if dir.IsDirectory(event.Name) {
					m.createStack(ctx, event.Name)
				}
			case fsnotify.Remove:
				m.removeStack(ctx, event.Name)
			}
		case <-ctx.Done():
			log.Info("manifest watcher done")
			return nil
		}
	}
}

func (m *Manager) createStack(ctx context.Context, name string) {
	// safeguard in case the fswatcher would trigger an event for an already existing stack
	if _, ok := m.stacks[name]; ok {
		return
	}

	stackCtx, cancelStack := context.WithCancel(ctx)
	stack := stack{cancelStack, NewStackApplier(name, m.KubeClientFactory)}
	m.stacks[name] = stack

	go func() {
		log := m.log.WithField("stack", name)
		for {
			log.Info("Running stack")
			if err := stack.Run(stackCtx); err != nil {
				log.WithError(err).Error("Failed to run stack")
			}

			select {
			case <-time.After(10 * time.Second):
				continue
			case <-stackCtx.Done():
				log.Info("Stack done")
				return
			}
		}
	}()
}

func (m *Manager) removeStack(ctx context.Context, name string) {
	stack, ok := m.stacks[name]
	if !ok {
		m.log.
			WithField("path", name).
			Debug("attempted to remove non-existent stack, probably not a directory")
		return
	}

	delete(m.stacks, name)
	stack.CancelFunc()

	log := m.log.WithField("stack", name)
	if err := stack.DeleteStack(ctx); err != nil {
		log.WithError(err).Error("Failed to delete stack")
		return
	}

	log.Info("Stack deleted successfully")
}
