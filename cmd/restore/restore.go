//go:build !windows
// +build !windows

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
	"os"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/backup"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type CmdOpts config.CLIOptions

var restoredConfigPath string

func NewRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore k0s state from given backup archive. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			if len(args) != 1 {
				return fmt.Errorf("path to backup archive expected")
			}
			return c.restore(args[0])
		},
		PreRunE: preRunValidateConfig,
	}

	cmd.SilenceUsage = true
	cmd.Flags().StringVar(&restoredConfigPath, "config-out", "/etc/k0s/k0s.yaml", "Specify desired name and full path for the restored k0s.yaml file (default: /etc/k0s/k0s.yaml)")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) restore(archivePath string) error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	k0sStatus, _ := install.GetStatusInfo(config.StatusSocket)
	if k0sStatus != nil && k0sStatus.Pid != 0 {
		logger.Fatal("k0s seems to be running! k0s must be down during the restore operation.")
	}

	if !file.Exists(archivePath) {
		return fmt.Errorf("given file %s does not exist", archivePath)
	}

	if !dir.IsDirectory(c.K0sVars.DataDir) {
		if err := dir.Init(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
			return err
		}
	}

	mgr, err := backup.NewBackupManager()
	if err != nil {
		return err
	}

	return mgr.RunRestore(archivePath, c.K0sVars, restoredConfigPath)
}

// TODO Need to move to some common place, now it's defined in restore and backup commands
func preRunValidateConfig(cmd *cobra.Command, args []string) error {
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
