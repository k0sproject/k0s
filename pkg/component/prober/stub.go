package prober

import "context"

// NopProber is a no-op prober
type NopProber struct{}

func (p NopProber) Run(context.Context)  {}
func (p NopProber) Register(string, any) {}
