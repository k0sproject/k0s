package cleanup

type bridge struct {
}

// Name returns the name of the step
func (b *bridge) Name() string {
	return "kube-bridge leftovers cleanup step"
}

// NeedsToRun checks if there are and kube-bridge leftovers
func (b *bridge) NeedsToRun() bool {
	return false
}

// Run removes found kube-bridge leftovers
func (b *bridge) Run() error {
	return nil
}
