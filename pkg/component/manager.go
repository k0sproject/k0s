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
package component

import (
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	components []Component
	sync       map[string]bool
}

// NewManager creates a manager
func NewManager() *Manager {
	return &Manager{
		components: []Component{},
	}
}

// Add adds a component to the manager
func (m *Manager) Add(component Component) {
	m.components = append(m.components, component)
}

// AddSync adds a component to the manager that should be initialized synchronously
func (m *Manager) AddSync(component Component) {
	m.components = append(m.components, component)
	compName := reflect.TypeOf(component).Elem().Name()
	if m.sync == nil {
		m.sync = make(map[string]bool)
	}
	m.sync[compName] = true
}

// Init initializes all managed components
func (m *Manager) Init() error {
	g := new(errgroup.Group)

	for _, comp := range m.components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("initializing %v\n", compName)
		c := comp
		if m.sync[compName] {
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
func (m *Manager) Start() error {
	for _, comp := range m.components {
		compName := reflect.TypeOf(comp).Elem().Name()
		logrus.Infof("starting %v", compName)
		if err := comp.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops all managed components
func (m *Manager) Stop() error {
	var ret error = nil
	for i := len(m.components) - 1; i >= 0; i-- {
		if err := m.components[i].Stop(); err != nil {
			logrus.Errorf("failed to stop component: %s", err.Error())
			if ret == nil {
				ret = fmt.Errorf("failed to stop components")
			}
		}
	}
	return ret
}
