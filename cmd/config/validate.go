/*
Copyright 2020 k0s authors

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

package config

import (
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate k0s configuration",
		Long: `Example:
   k0s config validate --config path_to_config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := config.GetCmdOpts()

			loadingRules := config.ClientConfigLoadingRules{K0sVars: c.K0sVars}
			_, err := loadingRules.ParseRuntimeConfig()
			return err
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	return cmd
}
