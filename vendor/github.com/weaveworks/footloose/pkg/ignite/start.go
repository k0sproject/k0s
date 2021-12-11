package ignite

import (
	"github.com/weaveworks/footloose/pkg/exec"
)

// Start starts an Ignite VM
func Start(name string) error {
	return exec.CommandWithLogging(execName, "start", name)
}
