/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package reset

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

func NewResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Helper command for uninstalling k0s. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			c := CmdOpts(config.GetCmdOpts())
			return c.reset()
		},
		PreRunE: preRunValidateConfig,
	}
	cmd.SilenceUsage = true
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) reset() error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		logger.Fatal("this command must be run as root!")
	}

	k0sStatus, _ := install.GetPid()
	if k0sStatus.Pid != 0 {
		logger.Fatal("k0s seems to be running! please stop k0s before reset.")
	}

	role := install.GetRoleByStagedKubelet(c.K0sVars.BinDir)
	logrus.Debugf("detected role for cleanup: %v", role)
	err := install.UninstallService(role)
	if err != nil {
		// might be that k0s was not run as a part of a service. just notify on uninstall error
		logger.Infof("failed to uninstall k0s service: %v", err)
	}
	// Get Cleanup Config
	cfg := install.NewCleanUpConfig(c.K0sVars.DataDir)

	if strings.Contains(role, "controller") {
		clusterConfig, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars)
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

func preRunValidateConfig(cmd *cobra.Command, args []string) error {
	c := CmdOpts(config.GetCmdOpts())
	_, err := config.ValidateYaml(c.CfgFile, c.K0sVars)
	if err != nil {
		return err
	}
	return nil
}
