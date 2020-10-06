package component

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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
	logrus.Debug("********Starting init")

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(len(m.components))
	errors := make(chan error)
	wgDone := make(chan bool)

	for _, comp := range m.components {
		go execute(comp, errors, wgDone, &wg)
	}

	go wait(wgDone, &wg)

	select {
	case <-wgDone:
		break
	case err := <-errors:
		fmt.Println("Error: ", err)
		close(errors)
		break
	}

	logrus.Debug("********finished init:", time.Since(start))
	return nil
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
	for i := len(m.components) - 1; i >= 0; i-- {
		m.components[i].Stop()
	}
	return nil
}

func execute(c Component, fatalErrors chan<- error, wgDone chan<- bool, wg *sync.WaitGroup) {
	err := c.Init()
	if err != nil {
		fatalErrors <- err
	}
	wg.Done()
}

func wait(wgDone chan<- bool, wg *sync.WaitGroup) {
	wg.Wait()
	close(wgDone)
}
