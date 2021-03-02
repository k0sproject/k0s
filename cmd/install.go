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
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

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
			flagsAndVals := []string{"controller"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			return setup("controller", flagsAndVals)
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
			flagsAndVals := []string{"worker"}
			flagsAndVals = append(flagsAndVals, cmdFlagsToArgs(cmd)...)
			return setup("worker", flagsAndVals)
		},
		PreRunE: preRunValidateConfig,
	}
)

// the setup functions:
// * Ensures that the proper users are created
// * sets up startup and logging for k0s
func setup(role string, args []string) error {
	if os.Geteuid() != 0 {
		logrus.Fatal("this command must be run as root!")
	}

	if role == "controller" {
		clusterConfig, err := ConfigFromYaml(cfgFile)
		if err != nil {
			logrus.Errorf("failed to get cluster setup: %v", err)
		}
		if err := install.CreateControllerUsers(clusterConfig, k0sVars); err != nil {
			logrus.Errorf("failed to create controller users: %v", err)
		}
	}

	err := install.EnsureService(args)
	if err != nil {
		logrus.Errorf("failed to install k0s service: %v", err)
	}
	return nil
}

func preRunValidateConfig(cmd *cobra.Command, args []string) error {
	return validateConfig(cfgFile)
}
