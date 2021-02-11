package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/install"
)

func init() {
	resetCmd.AddCommand(resetControllerCmd)
	resetCmd.AddCommand(resetWorkerCmd)
}

var (
	resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Helper command for uninstalling k0s. Must be run as root (or with sudo)",
	}
)

var (
	resetControllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "Helper command for uninstalling k0s controller node. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reset("controller")
		},
	}

	resetWorkerCmd = &cobra.Command{
		Use:   "worker",
		Short: "Helper command for uninstalling k0s worker node. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return reset("worker")
		},
	}
)

func reset(role string) error {
	if os.Geteuid() != 0 {
		logrus.Fatal("this command must be run as root!")
	}
	err := install.UninstallService(role)
	if err != nil {
		logrus.Errorf("failed to uninstall k0s service: %v", err)
	}

	if role == "controller" {
		clusterConfig, err := ConfigFromYaml(cfgFile)
		if err != nil {
			logrus.Errorf("failed to get cluster setup: %v", err)
		}
		if err := install.DeleteControllerUsers(clusterConfig); err != nil {
			// don't fail, just notify on delete error
			logrus.Infof("failed to delete controller users: %v", err)
		}
	}
	return nil
}
