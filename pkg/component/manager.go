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
package component

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	Components []Component
	sync       map[string]struct{}
}

// NewManager creates a manager
func NewManager() *Manager {
	return &Manager{
		Components: []Component{},
		sync:       map[string]struct{}{},
	}
}

// Add adds a component to the manager
func (m *Manager) Add(component Component) {
	m.Components = append(m.Components, component)
}

// AddSync adds a component to the manager that should be initialized synchronously
func (m *Manager) AddSync(component Component) {
	m.Components = append(m.Components, component)
	compName := reflect.TypeOf(component).Elem().Name()
	m.sync[compName] = struct{}{}
}

// Init initializes all managed components
func (m *Manager) Init() error {
	var g errgroup.Group

	for _, comp := range m.Components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("initializing %v\n", compName)
		c := comp
		if _, found := m.sync[compName]; found {
			if err := c.Init(); err != nil {
				return err
			}
		} else {
			// init this async
			g.Go(c.Init)
		}
	}
	err := g.Wait()
	return err
}

// Start starts all managed components
func (m *Manager) Start(ctx context.Context) error {
	perfTimer := performance.NewTimer("component-start").Buffer().Start()
	for _, comp := range m.Components {
		compName := reflect.TypeOf(comp).Elem().Name()
		perfTimer.Checkpoint(fmt.Sprintf("running-%s", compName))
		logrus.Infof("starting %v", compName)
		if err := comp.Run(); err != nil {
			return err
		}
		perfTimer.Checkpoint(fmt.Sprintf("running-%s-done", compName))
		if err := waitForHealthy(ctx, comp, compName); err != nil {
			return err
		}
	}
	perfTimer.Output()
	return nil
}

// Stop stops all managed components
func (m *Manager) Stop() error {
	var ret error
	for i := len(m.Components) - 1; i >= 0; i-- {
		if err := m.Components[i].Stop(); err != nil {
			logrus.Errorf("failed to stop component: %s", err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to stop components")
			}
		}
	}
	return ret
}

// Reoncile reconciles all managed components
func (m *Manager) Reconcile() error {
	var ret error
	for i := len(m.Components) - 1; i >= 0; i-- {
		if err := m.Components[i].Reconcile(); err != nil {
			logrus.Errorf("failed to reconcile component: %s", err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to reconcile components")
			}
		}
	}
	return ret
}

// waitForHealthy waits until the component is healthy and returns true upon success. If a timeout occurs, it returns false
func waitForHealthy(ctx context.Context, comp Component, name string) error {
	ctx, cancelFunction := context.WithTimeout(ctx, 2*time.Minute)

	// clear up context after timeout
	defer cancelFunction()

	// loop forever, until the context is canceled or until etcd is healthy
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			logrus.Debugf("checking %s for health", name)
			if err := comp.Healthy(); err != nil {
				logrus.Errorf("health-check: %s might be down: %v", name, err)
				continue
			}
			logrus.Debugf("%s is healthy. closing check", name)
			return nil
		case <-ctx.Done():
			return fmt.Errorf("%s health-check timed out", name)
		}
	}
}
