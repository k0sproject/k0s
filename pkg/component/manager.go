package component

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Manager manages components
type Manager struct {
	components []Component
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

// Init initializes all managed components
func (m *Manager) Init() error {
	g := new(errgroup.Group)

	for _, comp := range m.components {
		c := comp
		g.Go(c.Init)
	}
	err := g.Wait()
	return err
}

// Start starts all managed components
func (m *Manager) Start() error {
	for _, comp := range m.components {
		if err := comp.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops all managed components
func (m *Manager) Stop() error {
	errors := make([]error, 0, len(m.components))
	for i := len(m.components) - 1; i >= 0; i-- {
		if err := m.components[i].Stop(); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) != 0 {
		for _, err := range errors {
			logrus.Errorf("failed to stop component: %s", err.Error())
		}
		return fmt.Errorf("failed to stop components")
	}
	return nil
}
