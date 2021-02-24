package cmd

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/install"
)

func init() {
	addPersistentFlags(resetCmd)
}

var (
	resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Helper command for uninstalling k0s. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reset()
		},
	}
)

func reset() error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		logger.Fatal("this command must be run as root!")
	}

	k0sStatus, _ := getPid()
	if k0sStatus.Pid != 0 {
		logger.Fatal("k0s seems to be running! please stop k0s before reset.")
	}

	role := install.GetRoleByStagedKubelet(k0sVars.BinDir)
	logrus.Debugf("detected role for cleanup: %v", role)
	err := install.UninstallService(role)
	if err != nil {
		logger.Errorf("failed to uninstall k0s service: %v", err)
	}
	// Get Cleanup Config
	cfg := install.NewCleanUpConfig(k0sVars.DataDir)

	if strings.Contains(role, "controller") {
		clusterConfig, err := ConfigFromYaml(cfgFile)
		if err != nil {
			logger.Errorf("failed to get cluster setup: %v", err)
		}
		if err := install.DeleteControllerUsers(clusterConfig); err != nil {
			// don't fail, just notify on delete error
			logger.Infof("failed to delete controller users: %v", err)
		}
	}

	if strings.Contains(role, "worker") {
		if err := cfg.WorkerCleanup(); err != nil {
			logger.Infof("error while attempting to clean up worker resources: %v", err)
		}
	}

	if err := cfg.RemoveAllDirectories(); err != nil {
		logger.Info(err.Error())
	}
	logrus.Info("k0s cleanup operations done. To ensure a full reset, a node reboot is recommended.")
	return nil
}
