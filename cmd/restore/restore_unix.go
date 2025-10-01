//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package restore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/backup"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command struct {
	*config.CLIOptions
	restoredConfigPath string
}

func NewRestoreCmd() *cobra.Command {
	var (
		debugFlags         internal.DebugFlags
		restoredConfigPath string
	)

	cmd := &cobra.Command{
		Use:              "restore filename",
		Short:            "restore k0s state from given backup archive. Use '-' as filename to read from stdin. Must be run as root (or with sudo)",
		Args:             cobra.ExactArgs(1),
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			c := command{opts, restoredConfigPath}

			return c.restore(args[0], cmd.OutOrStdout())
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&restoredConfigPath, "config-out", "", "Specify desired name and full path for the restored k0s.yaml file (default: k0s_<archive timestamp>.yaml")

	return cmd
}

func (c *command) restore(path string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("this command must be run as root")
	}

	k0sStatus, _ := status.GetStatusInfo(c.K0sVars.StatusSocketPath)
	if k0sStatus != nil && k0sStatus.Pid != 0 {
		logrus.Fatal("k0s seems to be running! k0s must be down during the restore operation.")
	}

	if path != "-" && !file.Exists(path) {
		return fmt.Errorf("given file %s does not exist", path)
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
	if c.restoredConfigPath == "" {
		c.restoredConfigPath = defaultConfigFileOutputPath(path)
	}
	return mgr.RunRestore(path, c.K0sVars, c.restoredConfigPath, out)
}

// set output config file name and path according to input archive Timestamps
// the default location for the restore operation is the currently running cwd
// this can be override, by using the --config-out flag
func defaultConfigFileOutputPath(archivePath string) string {
	if archivePath == "-" {
		return "-"
	}
	f := filepath.Base(archivePath)
	nameWithoutExt := strings.Split(f, ".")[0]
	fName := strings.TrimPrefix(nameWithoutExt, "k0s_backup_")
	return fmt.Sprintf("k0s_%s.yaml", fName)
}
