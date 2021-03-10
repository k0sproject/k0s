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
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/install"
)

func init() {
	installCmd.AddCommand(installControllerCmd)
	installCmd.AddCommand(installWorkerCmd)

	addPersistentFlags(installControllerCmd)
	addPersistentFlags(installWorkerCmd)

	installControllerCmd.Flags().AddFlagSet(controllerCmd.Flags())
	installWorkerCmd.Flags().AddFlagSet(workerCmd.Flags())
	addPersistentFlags(installCmd)
}

var (
	installCmd = &cobra.Command{
		Use:   "install",
		Short: "Helper command for setting up k0s on a brand-new system. Must be run as root (or with sudo)",
	}
)

var (
	installControllerCmd = &cobra.Command{
		Use:     "controller",
		Short:   "Helper command for setting up k0s as controller node on a brand-new system. Must be run as root (or with sudo)",
		Aliases: []string{"server"},
		Example: `All default values of controller command will be passed to the service stub unless overriden. 

With controller subcommand you can setup a single node cluster by running:

	k0s install controller --enable-worker
	`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := convertFileParamsToAbsolute(); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			flagsAndVals := []string{"controller"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			if err := setup("controller", flagsAndVals); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			return nil
		},
		PreRunE: preRunValidateConfig,
	}

	installWorkerCmd = &cobra.Command{
		Use:   "worker",
		Short: "Helper command for setting up k0s as a worker node on a brand-new system. Must be run as root (or with sudo)",
		Example: `Worker subcommand allows you to pass in all available worker parameters. 
All default values of worker command will be passed to the service stub unless overriden.

Windows flags like "--api-server", "--cidr-range" and "--cluster-dns" will be ignored since install command doesn't yet support Windows services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := convertFileParamsToAbsolute(); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			flagsAndVals := []string{"worker"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			if err := setup("worker", flagsAndVals); err != nil {
				cmd.SilenceUsage = true
				return err
			}

			return nil
		},
		PreRunE: preRunValidateConfig,
	}
)

// the setup functions:
// * Ensures that the proper users are created
// * sets up startup and logging for k0s
func setup(role string, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	// if cfgFile is not provided k0s will handle this so no need to check if the file exists.
	if cfgFile != "" && !util.IsDirectory(cfgFile) && !util.FileExists(cfgFile) {
		return fmt.Errorf("file %s does not exist", cfgFile)
	}

	if role == "controller" {
		clusterConfig, err := ConfigFromYaml(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to get cluster setup: %v", err)
		}
		if err := install.CreateControllerUsers(clusterConfig, k0sVars); err != nil {
			return fmt.Errorf("failed to create controller users: %v", err)
		}
	}

	err := install.EnsureService(args)
	if err != nil {
		return fmt.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

func convertFileParamsToAbsolute() (err error) {
	// don't convert if cfgFile is empty
	if cfgFile != "" {
		cfgFile, err = filepath.Abs(cfgFile)
		if err != nil {
			return err
		}
	}

	if dataDir != "" {
		dataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return err
		}
	}

	if tokenFile != "" {
		tokenFile, err = filepath.Abs(tokenFile)
		if err != nil {
			return err
		}
		if !util.FileExists(tokenFile) {
			return fmt.Errorf("%s does not exist", tokenFile)
		}
	}

	return nil
}

func preRunValidateConfig(cmd *cobra.Command, args []string) error {
	return validateConfig(cfgFile)
}
