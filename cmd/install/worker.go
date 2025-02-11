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

			flagsAndVals, err := cmdFlagsToArgs(cmd)
			if err != nil {
				return err
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
