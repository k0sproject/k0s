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

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

type CmdOpts config.CLIOptions

func NewInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)",
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
		c.Logger.Fatal("this command must be run as root!")
	}

	// if cfgFile is not provided k0s will handle this so no need to check if the file exists.
	if c.CfgFile != "" && !util.IsDirectory(c.CfgFile) && !util.FileExists(c.CfgFile) {
		c.Logger.Fatalf("file %s does not exist", c.CfgFile)
	}
	if role == "controller" {
		cfg, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars, c.Logger)
		if err != nil {
			return err
		}
		c.ClusterConfig = cfg
		if err := install.CreateControllerUsers(c.ClusterConfig, c.K0sVars); err != nil {
			c.Logger.Fatalf("failed to create controller users: %v", err)
		}
	}
	err := install.EnsureService(args)
	if err != nil {
		c.Logger.Fatalf("failed to install k0s service: %v", err)
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
		if !util.FileExists(c.TokenFile) {
			return fmt.Errorf("%s does not exist", c.TokenFile)
		}
	}
	return nil
}

func preRunValidateConfig(_ *cobra.Command, _ []string) error {
	c := CmdOpts(config.GetCmdOpts())
	_, err := config.ValidateYaml(c.CfgFile, c.K0sVars)
	c.Logger = util.CLILogger()
	if err != nil {
		return err
	}
	return nil
}
