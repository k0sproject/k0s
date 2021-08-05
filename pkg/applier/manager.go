/*
Copyright 2021 k0s authors

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

	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	K0sVars           constant.CfgVars
	KubeClientFactory kubeutil.ClientFactoryInterface

	// client               kubernetes.Interface
	applier       Applier
	bundlePath    string
	cancelWatcher context.CancelFunc
	log           *logrus.Entry
	stacks        map[string]*StackApplier

	LeaderElector controller.LeaderElector
}

// Init initializes the Manager
func (m *Manager) Init() error {
	err := dir.Init(m.K0sVars.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return fmt.Errorf("failed to create manifest bundle dir %s: %w", m.K0sVars.ManifestsDir, err)
	}
	m.log = logrus.WithField("component", "applier-manager")
	m.stacks = make(map[string]*StackApplier)
	m.bundlePath = m.K0sVars.ManifestsDir

	m.applier = NewApplier(m.K0sVars.ManifestsDir, m.KubeClientFactory)

	m.LeaderElector.AddAcquiredLeaseCallback(func() {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancelWatcher = cancel
		go func() {
			_ = m.runWatchers(ctx)
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
func (m *Manager) Run() error {
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

	dirs, err := dir.GetAll(m.bundlePath)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if err := m.createStack(path.Join(m.bundlePath, dir)); err != nil {
			log.WithError(err).Error("failed to create stack")
			return err
		}
	}

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
					if err := m.createStack(event.Name); err != nil {
						return err
					}
				}
			case fsnotify.Remove:
				_ = m.removeStack(event.Name)
			}
		case <-ctx.Done():
			log.Info("manifest watcher done")
			return nil
		}
	}
}

func (m *Manager) createStack(name string) error {
	// safeguard in case the fswatcher would trigger an event for an already existing watcher
	if _, ok := m.stacks[name]; ok {
		return nil
	}
	m.log.WithField("stack", name).Info("registering new stack")
	sa, err := NewStackApplier(name, m.KubeClientFactory)
	if err != nil {
		return err
	}

	go func() {
		_ = sa.Start()
	}()

	m.stacks[name] = sa
	return nil
}

func (m *Manager) removeStack(name string) error {
	sa, ok := m.stacks[name]

	if !ok {
		m.log.
			WithField("path", name).
			Debug("attempted to remove non-existent stack, probably not a directory")
		return nil
	}
	err := sa.Stop()
	if err != nil {
		m.log.WithField("stack", name).WithError(err).Warn("failed to stop stack applier")
		return err
	}
	err = sa.DeleteStack()
	if err != nil {
		m.log.WithField("stack", name).WithError(err).Warn("failed to stop and delete a stack applier")
		return err
	}
	m.log.WithField("stack", name).Info("stack deleted succesfully")
	delete(m.stacks, name)

	return nil
}

// Health-check interface
func (m *Manager) Healthy() error { return nil }
