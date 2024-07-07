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

	"github.com/k0sproject/k0s/pkg/cleanup"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command config.CLIOptions

func NewResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall k0s. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS == "windows" {
				return fmt.Errorf("currently not supported on windows")
			}
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			c := (*command)(opts)
			return c.reset()
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.GetCriSocketFlag())
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	return cmd
}

func (c *command) reset() error {
	if os.Geteuid() != 0 {
		logrus.Fatal("this command must be run as root!")
	}

	k0sStatus, _ := status.GetStatusInfo(c.K0sVars.StatusSocketPath)
	if k0sStatus != nil && k0sStatus.Pid != 0 {
		logrus.Fatal("k0s seems to be running! please stop k0s before reset.")
	}

	nodeCfg, err := c.K0sVars.NodeConfig()
	if err != nil {
		return err
	}
	if nodeCfg.Spec.Storage.Kine != nil && nodeCfg.Spec.Storage.Kine.DataSource != "" {
		logrus.Warn("Kine dataSource is configured. k0s will not reset the data source if it points to an external database. If you plan to continue using the data source, you should reset it to avoid conflicts.")
	}

	// Get Cleanup Config
	cfg, err := cleanup.NewConfig(c.K0sVars, c.CfgFile, c.WorkerOptions.CriSocket)
	if err != nil {
		return fmt.Errorf("failed to configure cleanup: %w", err)
	}

	err = cfg.Cleanup()
	logrus.Info("k0s cleanup operations done.")
	logrus.Warn("To ensure a full reset, a node reboot is recommended.")

	return err
}
