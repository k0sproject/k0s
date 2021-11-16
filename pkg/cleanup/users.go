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
	logger := logrus.New()
	// get k0s config
	loadingRules := config.ClientConfigLoadingRules{Nodeconfig: true}
	cfg, err := loadingRules.Load()
	if err != nil {
		logger.Errorf("failed to get cluster setup: %v", err)
	}

	if err := install.DeleteControllerUsers(cfg); err != nil {
		// don't fail, just notify on delete error
		logger.Infof("failed to delete controller users: %v", err)
	}
	return nil
}
