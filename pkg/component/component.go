package component

type Component interface {
	Init() error
	Run() error
	Stop() error
}
