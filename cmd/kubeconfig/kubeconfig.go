/*
Copyright 2022 k0s authors

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

package kubeconfig

import (
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/spf13/cobra"
)

type CmdOpts config.CLIOptions

func NewKubeConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig [command]",
		Short: "Create a kubeconfig file for a specified user",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.SilenceUsage = true
	cmd.AddCommand(kubeconfigCreateCmd())
	cmd.AddCommand(kubeConfigAdminCmd())
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
