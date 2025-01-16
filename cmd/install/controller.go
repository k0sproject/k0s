//go:build linux

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
	"errors"
	"fmt"
	"os"
	"testing/iotest"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/spf13/cobra"
)

func installControllerCmd(installFlags *installFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "controller",
		Short:   "Install k0s controller on a brand-new system. Must be run as root (or with sudo)",
		Aliases: []string{"server"},
		Example: `All default values of controller command will be passed to the service stub unless overridden.

With the controller subcommand you can setup a single node cluster by running:

	k0s install controller --single
	`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if os.Geteuid() != 0 {
				return errors.New("this command must be run as root")
			}

			cmd.SetIn(iotest.ErrReader(errors.New("cannot read configuration from standard input when installing k0s")))
			k0sVars, err := config.NewCfgVars(cmd)
			if err != nil {
				return fmt.Errorf("failed to initialize configuration variables: %w", err)
			}

			nodeConfig, err := k0sVars.NodeConfig()
			if err != nil {
				return fmt.Errorf("failed to load node config: %w", err)
			}

			if errs := nodeConfig.Validate(); len(errs) > 0 {
				return fmt.Errorf("invalid node config: %w", errors.Join(errs...))
			}

			flagsAndVals, err := cmdFlagsToArgs(cmd)
			if err != nil {
				return err
			}

			systemUsers := nodeConfig.Spec.Install.SystemUsers
			homeDir := k0sVars.DataDir
			if err := install.EnsureControllerUsers(systemUsers, homeDir); err != nil {
				return fmt.Errorf("failed to create controller users: %w", err)
			}

			args := append([]string{"controller"}, flagsAndVals...)
			if err := install.InstallService(args, installFlags.envVars, installFlags.force); err != nil {
				return fmt.Errorf("failed to install controller service: %w", err)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetControllerFlags(&config.ControllerOptions{}))
	flags.AddFlagSet(config.GetWorkerFlags())
	flags.AddFlagSet(config.FileInputFlag())

	return cmd
}
