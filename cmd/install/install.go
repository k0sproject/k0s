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

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

func NewInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install k0s on a brand-new system. Must be run as root (or with sudo)",
	}

	cmd.AddCommand(installControllerCmd())
	cmd.AddCommand(installWorkerCmd())
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

// the setup functions:
// * Ensures that the proper users are created
// * sets up startup and logging for k0s
func (c *CmdOpts) setup(role string, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	// if cfgFile is not provided k0s will handle this so no need to check if the file exists.
	if c.CfgFile != "" && !dir.IsDirectory(c.CfgFile) && !file.Exists(c.CfgFile) {
		return fmt.Errorf("file %s does not exist", c.CfgFile)
	}
	if role == "controller" {
		// get k0s config
		loadingRules := config.ClientConfigLoadingRules{Nodeconfig: true}
		cfg, err := loadingRules.ParseRuntimeConfig()
		if err != nil {
			return err
		}
		c.NodeConfig = cfg
		if err := install.CreateControllerUsers(c.NodeConfig, c.K0sVars); err != nil {
			return fmt.Errorf("failed to create controller users: %v", err)
		}
	}
	err := install.EnsureService(args)
	if err != nil {
		return fmt.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

// this command converts the file paths in the Cmd Opts struct to Absolute Paths
// for flags passed to service init file, see the cmdFlagsToArgs func
func (c *CmdOpts) convertFileParamsToAbsolute() (err error) {
	// don't convert if cfgFile is empty

	if c.CfgFile != "" {
		c.CfgFile, err = filepath.Abs(c.CfgFile)
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

func preRunValidateConfig(_ *cobra.Command, _ []string) error {
	c := CmdOpts(config.GetCmdOpts())

	// get k0s config
	loadingRules := config.ClientConfigLoadingRules{}
	cfg, err := loadingRules.ParseRuntimeConfig()
	if err != nil {
		return fmt.Errorf("error in config loading: %v", err)
	}

	_, err = loadingRules.Validate(cfg, c.K0sVars)
	if err != nil {
		return fmt.Errorf("error in config validation: %v", err)
	}
	return nil
}
