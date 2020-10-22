package component

// Component defines the interface each managed component implements
type Component interface {
	Init() error
	Run() error
	Stop() error
	Healthy() error
}
