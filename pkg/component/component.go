package component

type Component interface {
	Run() error
	Stop() error
}
