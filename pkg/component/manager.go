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
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	Components []Component

	lastReconciledConfig *v1beta1.ClusterConfig
}

// NewManager creates a manager
func NewManager() *Manager {
	return &Manager{
		Components: []Component{},
	}
}

// Add adds a component to the manager
func (m *Manager) Add(ctx context.Context, component Component) {
	m.Components = append(m.Components, component)
	if isReconcileComponent(component) && m.lastReconciledConfig != nil {
		if err := m.reconcileComponent(ctx, component, m.lastReconciledConfig); err != nil {
			logrus.Warnf("component reconciler failed: %v", err)
		}
	}
}

// Init initializes all managed components
func (m *Manager) Init(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)

	for _, comp := range m.Components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("initializing %v\n", compName)
		c := comp
		// init this async
		g.Go(func() error {
			return c.Init(ctx)
		})
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
		if err := comp.Run(ctx); err != nil {
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
	for _, component := range m.Components {
		compName := reflect.TypeOf(component).Elem().Name()
		logrus.Infof("stopping component %s", compName)
		if err := component.Stop(); err != nil {
			logrus.Errorf("failed to stop component: %s", err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to stop components")
			}
		}
		logrus.Infof("stopped component %s", compName)
	}
	return ret
}

// ReconcileError is just a wrapper for possible many errors
type ReconcileError struct {
	Errors []error
}

// Error returns the stringified error message
func (r ReconcileError) Error() string {
	messages := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		messages[i] = e.Error()
	}
	return strings.Join(messages, "\n")
}

// Reconcile reconciles all managed components
func (m *Manager) Reconcile(ctx context.Context, cfg *v1beta1.ClusterConfig) error {
	errors := make([]error, 0)
	var ret error
	logrus.Infof("starting component reconciling for %d components", len(m.Components))
	for _, component := range m.Components {
		if err := m.reconcileComponent(ctx, component, cfg); err != nil {
			errors = append(errors, err)
		}
	}
	m.lastReconciledConfig = cfg
	if len(errors) > 0 {
		ret = ReconcileError{
			Errors: errors,
		}
	}
	logrus.Debugf("all component reconciled, result: %v", ret)
	return ret
}

func (m *Manager) reconcileComponent(ctx context.Context, component Component, cfg *v1beta1.ClusterConfig) error {
	clusterComponent, ok := component.(ReconcilerComponent)
	compName := reflect.TypeOf(component).String()
	if !ok {
		logrus.Debugf("%s does not implement the ReconcileComponent interface --> not reconciling it", compName)
		return nil
	}
	logrus.Infof("starting to reconcile %s", compName)
	if err := clusterComponent.Reconcile(ctx, cfg); err != nil {
		logrus.Errorf("failed to reconcile component %s: %s", compName, err.Error())
		return err
	}
	return nil
}

func isReconcileComponent(component Component) bool {
	_, ok := component.(ReconcilerComponent)
	return ok
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
