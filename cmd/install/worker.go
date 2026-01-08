// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/install"
)

func installWorkerCmd(installFlags *installFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Install k0s worker on a brand-new system. Must be run as root (or with sudo)",
		Example: `Worker subcommand allows you to pass in all available worker parameters.
All default values of worker command will be passed to the service stub unless overridden.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "windows" && os.Geteuid() != 0 {
				return errors.New("this command must be run as root")
			}

			envVars, err := resolveEnvVars(installFlags.envVars)
			if err != nil {
				return err
			}

			flagsAndVals, err := cmdFlagsToArgs(cmd)
			if err != nil {
				return err
			}

			args := append([]string{"worker"}, flagsAndVals...)
			if err := install.InstallService(args, envVars, installFlags.force); err != nil {
				return fmt.Errorf("failed to install worker service: %w", err)
			}

			if installFlags.start {
				if err := install.StartInstalledService(installFlags.force); err != nil {
					return fmt.Errorf("failed to start worker service: %w", err)
				}
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetWorkerFlags())

	return cmd
}
