package cleanup

import (
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
	"github.com/sirupsen/logrus"
)

type users struct {
	Config *Config
}

// Name returns the name of the step
func (u *users) Name() string {
	return "remove k0s users step:"
}

// Run removes all controller users that are present on the host
func (u *users) Run() error {
	cfg, err := config.GetNodeConfig(u.Config.cfgFile, u.Config.k0sVars)
	if err != nil {
		logrus.Errorf("failed to get cluster setup: %v", err)
		return nil
	}
	if err := install.DeleteControllerUsers(cfg); err != nil {
		// don't fail, just notify on delete error
		logrus.Warnf("failed to delete controller users: %v", err)
	}
	return nil
}
