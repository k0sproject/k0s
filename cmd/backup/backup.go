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
package backup

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/backup"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

var savePath string

func NewBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back-Up k0s configuration. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			return c.backup()
		},
		PreRunE: preRunValidateConfig,
	}
	cmd.Flags().StringVar(&savePath, "save-path", "", "destination directory path for backup assets")
	cmd.SilenceUsage = true
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *CmdOpts) backup() error {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.ForceColors = true
	textFormatter.DisableTimestamp = true

	logger.SetFormatter(textFormatter)

	if os.Geteuid() != 0 {
		logger.Fatal("this command must be run as root!")
	}

	if !util.DirExists(savePath) {
		logger.Fatalf("the save-path directory (%v) does not exist.", savePath)
	}

	if !util.DirExists(c.K0sVars.DataDir) {
		logger.Fatalf("cannot find data-dir (%v). check your environment and/or command input and try again.", c.K0sVars.DataDir)
	}

	role := install.GetRoleByStagedKubelet(c.K0sVars.BinDir)
	logrus.Debugf("detected role for backup operations: %v", role)

	if strings.Contains(role, "controller") {
		clusterConfig, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars)
		if err != nil {
			logger.Errorf("failed to get cluster setup: %v", err)
		}
		mgr, err := backup.NewBackupManager(clusterConfig.Spec, c.K0sVars)
		if err != nil {
			return err
		}
		return mgr.RunBackup(savePath)
	}
	return fmt.Errorf("backup command must be run on the controller node, have `%s`", role)
}

func preRunValidateConfig(cmd *cobra.Command, args []string) error {
	c := CmdOpts(config.GetCmdOpts())
	_, err := config.ValidateYaml(c.CfgFile, c.K0sVars)
	if err != nil {
		return err
	}
	return nil
}
