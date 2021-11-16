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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/cleanup"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

func NewResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Uninstall k0s. Must be run as root (or with sudo)",
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
	cmd.Flags().AddFlagSet(config.GetCriSocketFlag())
	return cmd
}

func (c *CmdOpts) reset() error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		logger.Fatal("this command must be run as root!")
	}

	k0sStatus, _ := install.GetStatusInfo(config.StatusSocket)
	if k0sStatus != nil && k0sStatus.Pid != 0 {
		logger.Fatal("k0s seems to be running! please stop k0s before reset.")
	}

	// Get Cleanup Config
	cfg, err := cleanup.NewConfig(c.K0sVars, c.WorkerOptions.CriSocket)
	if err != nil {
		return fmt.Errorf("failed to configure cleanup: %v", err)
	}

	err = cfg.Cleanup()

	logger.Info("k0s cleanup operations done. To ensure a full reset, a node reboot is recommended.")
	return err
}

func preRunValidateConfig(_ *cobra.Command, _ []string) error {
	c := CmdOpts(config.GetCmdOpts())

	// get k0s config
	loadingRules := config.ClientConfigLoadingRules{}
	cfg, err := loadingRules.ParseRuntimeConfig()
	if err != nil {
		return err
	}

	_, err = loadingRules.Validate(cfg, c.K0sVars)
	if err != nil {
		return err
	}
	return nil
}
