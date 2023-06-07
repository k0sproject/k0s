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
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	var restoredConfigPath string

	cmd := &cobra.Command{
		Use:   "restore filename",
		Short: "restore k0s state from given backup archive. Use '-' as filename to read from stdin. Must be run as root (or with sudo)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			c := command{opts, restoredConfigPath}

			return c.restore(args[0], cmd.OutOrStdout())
		},
	}

	cmd.SilenceUsage = true

	cwd, err := os.Getwd()
	if err != nil {
		logrus.Fatal("failed to get local path")
	}

	restoredConfigPathDescription := fmt.Sprintf("Specify desired name and full path for the restored k0s.yaml file (default: %s/k0s_<archive timestamp>.yaml", cwd)
	cmd.Flags().StringVar(&restoredConfigPath, "config-out", "", restoredConfigPathDescription)
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *command) restore(path string, out io.Writer) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
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
	restoredFileName := fmt.Sprintf("k0s_%s.yaml", fName)

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return path.Join(cwd, restoredFileName)
}
