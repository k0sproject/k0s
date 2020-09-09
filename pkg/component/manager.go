package component

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
	for _, comp := range m.components {
		if err := comp.Init(); err != nil {
			return err
		}
	}
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
