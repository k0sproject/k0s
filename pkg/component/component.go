package component

// Component defines the interface each managed component implements
type Component interface {
	Init() error
	Run() error
	Stop() error
	Healthy() error
}

// Storage provides an interface so we can swap out the
// implementation of storage componentAdd between kine & etcd
type Storage interface {
	Component
}
