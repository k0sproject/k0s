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
package restore

import (
	"fmt"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/backup"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/install"

	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type CmdOpts config.CLIOptions

func NewRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore k0s state from given backup archive. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			if len(args) != 1 {
				return fmt.Errorf("path to backup archive expected")
			}
			cfg, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars)
			if err != nil {
				return err
			}

			c.ClusterConfig = cfg
			return c.restore(args[0])
		},
		PreRunE: preRunValidateConfig,
	}

	cmd.SilenceUsage = true
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) restore(path string) error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	k0sStatus, _ := install.GetPid()
	if k0sStatus.Pid != 0 {
		logger.Fatal("k0s seems to be running! k0s must be down during the restore operation.")
	}

	if !util.FileExists(path) {
		return fmt.Errorf("given file %s does not exist", path)
	}

	if !util.DirExists(c.K0sVars.DataDir) {
		if err := util.InitDirectory(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
			return err
		}
	}

	mgr, err := backup.NewBackupManager()
	if err != nil {
		return err
	}
	// c.CfgFile, c.ClusterConfig.Spec, c.K0sVars
	return mgr.RunRestore(path, c.K0sVars)
}

// TODO Need to move to some common place, now it's defined in restore and backup commands
func preRunValidateConfig(_ *cobra.Command, _ []string) error {
	c := CmdOpts(config.GetCmdOpts())
	_, err := config.ValidateYaml(c.CfgFile, c.K0sVars)
	if err != nil {
		return err
	}
	return nil
}
