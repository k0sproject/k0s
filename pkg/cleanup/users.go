package cleanup

import (
	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type users struct {
	Config *Config
}

// Name returns the name of the step
func (u *users) Name() string {
	return "remove k0s users step:"
}

// NeedsToRun detects controller users
func (u *users) NeedsToRun() bool {
	clusterConfig, err := config.GetYamlFromFile(u.Config.cfgFile, u.Config.k0sVars, util.CLILogger())
	if err != nil {
		return false
	}

	users := install.GetControllerUsers(clusterConfig)
	for _, user := range users {
		if exists, _ := util.CheckIfUserExists(user); exists {
			return true
		}
	}
	return false
}

// Run removes all controller users that are present on the host
func (u *users) Run() error {
	logger := util.CLILogger()
	clusterConfig, err := config.GetYamlFromFile(u.Config.cfgFile, u.Config.k0sVars, logger)
	if err != nil {
		logger.Errorf("failed to get cluster setup: %v", err)
	}
	if err := install.DeleteControllerUsers(clusterConfig); err != nil {
		// don't fail, just notify on delete error
		logger.Infof("failed to delete controller users: %v", err)
	}
	return nil
}
