package cleanup

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/install"
	"github.com/sirupsen/logrus"
)

type services struct {
	Config *Config
	roles  []string
}

// Name returns the name of the step
func (s *services) Name() string {
	return "uninstal service step"
}

// NeedsToRun checks if k0s service files are persent on the host
func (s *services) NeedsToRun() bool {
	possibleRoles := []string{
		"controller", "worker",
	}
	for _, prole := range possibleRoles {
		if _, stub, err := install.GetSysInit(prole); err == nil && stub != "" {
			s.roles = append(s.roles, prole)
		}
	}

	return len(s.roles) > 0
}

// Run uninstalls k0s services that are found on the host
func (s *services) Run() error {
	var msg []string
	for _, role := range s.roles {
		if err := install.UninstallService(role); err != nil {
			logrus.Debugf("Tried removing service: %v", err)
			msg = append(msg, err.Error())
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, "\n"))
	}
	return nil
}
