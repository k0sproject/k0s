//go:build linux

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

			envVars, err := resolveEnvVars(installFlags.envVars)
			if err != nil {
				return err
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
			if err := install.InstallService(args, envVars, installFlags.force); err != nil {
				return fmt.Errorf("failed to install controller service: %w", err)
			}

			if installFlags.start {
				if err := install.StartInstalledService(installFlags.force); err != nil {
					return fmt.Errorf("failed to start controller service: %w", err)
				}
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
