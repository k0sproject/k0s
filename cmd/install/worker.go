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

			k0sVars, err := config.NewCfgVars(cmd)
			if err != nil {
				return fmt.Errorf("failed to initialize configuration variables: %w", err)
			}

			// Convert --token-env to --token-file
			tokenFilePath, err := handleTokenEnv(cmd, k0sVars.DataDir)
			if err != nil {
				return err
			}

			flagsAndVals, err := cmdFlagsToArgs(cmd)
			if err != nil {
				return err
			}

			if tokenFilePath != "" {
				flagsAndVals = append(flagsAndVals, "--token-file="+tokenFilePath)
			}

			args := append([]string{"worker"}, flagsAndVals...)
			if err := install.InstallService(args, installFlags.envVars, installFlags.force); err != nil {
				return fmt.Errorf("failed to install worker service: %w", err)
			}

			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetWorkerFlags())

	return cmd
}
