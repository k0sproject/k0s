//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/backup"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command config.CLIOptions

func NewBackupCmd() *cobra.Command {
	var (
		debugFlags internal.DebugFlags
		savePath   string
	)

	cmd := &cobra.Command{
		Use:              "backup",
		Short:            "Back-Up k0s configuration. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			c := (*command)(opts)
			nodeConfig, err := c.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			if nodeConfig.Spec.Storage.Etcd.IsExternalClusterUsed() {
				return errors.New("command 'k0s backup' does not support external etcd cluster")
			}
			return c.backup(nodeConfig, savePath, cmd.OutOrStdout())
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&savePath, "save-path", "", "destination directory path for backup assets, use '-' for stdout")

	return cmd
}

func (c *command) backup(nodeConfig *k0sv1beta1.ClusterConfig, savePath string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("this command must be run as root")
	}

	if savePath != "-" && !dir.IsDirectory(savePath) {
		return fmt.Errorf("the save-path directory (%s) does not exist", savePath)
	}

	if !dir.IsDirectory(c.K0sVars.DataDir) {
		return fmt.Errorf("cannot find data-dir (%s). check your environment and/or command input and try again", c.K0sVars.DataDir)
	}

	status, err := status.GetStatusInfo(c.K0sVars.StatusSocketPath)
	if err != nil {
		return fmt.Errorf("unable to detect cluster status %w", err)
	}
	logrus.Debugf("detected role for backup operations: %v", status.Role)

	if strings.Contains(status.Role, "controller") {
		mgr, err := backup.NewBackupManager()
		if err != nil {
			return err
		}
		return mgr.RunBackup(nodeConfig.Spec, c.K0sVars, savePath, out)
	}
	return fmt.Errorf("backup command must be run on the controller node, have `%s`", status.Role)
}
