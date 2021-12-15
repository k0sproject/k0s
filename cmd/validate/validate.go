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
package validate

import (
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type CmdOpts config.CLIOptions

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validation related sub-commands",
	}
	cmd.AddCommand(validateConfigCmd())
	cmd.SilenceUsage = true
	return cmd
}

func validateConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Validate k0s configuration",
		Long: `Example:
   k0s validate config --config path_to_config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())

			loadingRules := config.ClientConfigLoadingRules{RuntimeConfigPath: c.CfgFile}
			_, err := loadingRules.Load()
			if err != nil {
				return err
			}
			logrus.Infof("no errors found in provided config file: %v", c.CfgFile)
			return nil
		},
		SilenceUsage: true,
	}

	// append flags
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	return cmd
}
