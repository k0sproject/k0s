/*
Copyright 2020 Mirantis, Inc.

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
	"path"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
)

// Manager is the Component interface wrapper for Applier
type Manager struct {
	K0sVars           constant.CfgVars
	KubeClientFactory kubeutil.ClientFactory

	//client               kubernetes.Interface
	applier              Applier
	bundlePath           string
	cancelLeaderElection context.CancelFunc
	cancelWatcher        context.CancelFunc
	log                  *logrus.Entry
	stacks               map[string]*StackApplier
}

// Init initializes the Manager
func (m *Manager) Init() error {
	err := util.InitDirectory(m.K0sVars.ManifestsDir, constant.ManifestsDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create manifest bundle dir %s", m.K0sVars.ManifestsDir)
	}
	m.log = logrus.WithField("component", "applier-manager")
	m.stacks = make(map[string]*StackApplier)
	m.bundlePath = m.K0sVars.ManifestsDir

	m.applier = NewApplier(m.K0sVars.ManifestsDir, m.KubeClientFactory)
	return err
}

// Run runs the Manager
func (m *Manager) Run() error {
	log := m.log
	kubeClient, err := m.KubeClientFactory.GetClient()
	if err != nil {
		return nil
	}

	leasePool, err := leaderelection.NewLeasePool(kubeClient, "k0s-manifest-applier", leaderelection.WithLogger(log))

	if err != nil {
		return err
	}

	electionEvents := &leaderelection.LeaseEvents{
		AcquiredLease: make(chan struct{}),
		LostLease:     make(chan struct{}),
	}

	go m.watchLeaseEvents(electionEvents)
	go func() {
		_, cancel, _ := leasePool.Watch(leaderelection.WithOutputChannels(electionEvents))
		m.cancelLeaderElection = cancel
	}()

	return nil
}

// Stop stops the Manager
func (m *Manager) Stop() error {
	if m.cancelLeaderElection != nil {
		m.cancelLeaderElection()
	}
	return nil
}

func (m *Manager) watchLeaseEvents(events *leaderelection.LeaseEvents) {
	log := m.log

	for {
		select {
		case <-events.AcquiredLease:
			log.Info("acquired leader lease")
			ctx, cancel := context.WithCancel(context.Background())
			m.cancelWatcher = cancel
			go func() {
				_ = m.runWatchers(ctx)
			}()
		case <-events.LostLease:
			log.Info("lost leader lease")
			if m.cancelWatcher != nil {
				m.cancelWatcher()
			}
		}
	}
}

func (m *Manager) runWatchers(ctx context.Context) error {
	log := logrus.WithField("component", "applier-manager")

	dirs, err := util.GetAllDirs(m.bundlePath)
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
				if util.IsDirectory(event.Name) {
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

	delete(m.stacks, name)

	return nil
}

// Health-check interface
func (m *Manager) Healthy() error { return nil }
