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

package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/spf13/cobra"
)

type command config.CLIOptions

type installFlags struct {
	force   bool
	envVars []string
}

func NewInstallCmd() *cobra.Command {
	var installFlags installFlags

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install k0s on a brand-new system. Must be run as root (or with sudo)",
	}

	cmd.AddCommand(installControllerCmd(&installFlags))
	cmd.AddCommand(installWorkerCmd(&installFlags))
	cmd.PersistentFlags().BoolVar(&installFlags.force, "force", false, "force init script creation")
	cmd.PersistentFlags().StringArrayVarP(&installFlags.envVars, "env", "e", nil, "set environment variable")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

// The setup functions:
//   - Ensures that the proper users are created.
//   - Sets up startup and logging for k0s.
func (c *command) setup(role string, args []string, installFlags *installFlags) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	nodeConfig, err := c.K0sVars.NodeConfig()
	if err != nil {
		return err
	}

	if role == "controller" {
		if err := install.CreateControllerUsers(nodeConfig, c.K0sVars); err != nil {
			return fmt.Errorf("failed to create controller users: %v", err)
		}
	}
	err = install.EnsureService(args, installFlags.envVars, installFlags.force)
	if err != nil {
		return fmt.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

// This command converts the file paths in the command struct to absolute paths.
// For flags passed to service init file, see the [cmdFlagsToArgs] func.
func (c *command) convertFileParamsToAbsolute() (err error) {
	// don't convert if cfgFile is empty

	if c.K0sVars.StartupConfigPath != "" {
		c.K0sVars.StartupConfigPath, err = filepath.Abs(c.K0sVars.StartupConfigPath)
		if err != nil {
			return err
		}
	}
	if c.K0sVars.DataDir != "" {
		c.K0sVars.DataDir, err = filepath.Abs(c.K0sVars.DataDir)
		if err != nil {
			return err
		}
	}
	if c.TokenFile != "" {
		c.TokenFile, err = filepath.Abs(c.TokenFile)
		if err != nil {
			return err
		}
		if !file.Exists(c.TokenFile) {
			return fmt.Errorf("%s does not exist", c.TokenFile)
		}
	}
	return nil
}
