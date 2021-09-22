package cleanup

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/k0sproject/k0s/pkg/install"
)

type services struct {
	Config *Config
}

// Name returns the name of the step
func (s *services) Name() string {
	return "uninstall service step"
}

// NeedsToRun checks if k0s service files are persent on the host
func (s *services) NeedsToRun() bool {
	return true
}

// Run uninstalls k0s services that are found on the host
func (s *services) Run() error {
	var msg []string

	for _, role := range []string{"controller", "worker"} {
		if err := install.UninstallService(role); err != nil && !errors.Is(err, fs.ErrNotExist) {
			msg = append(msg, err.Error())
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, "\n"))
	}
	return nil
}
