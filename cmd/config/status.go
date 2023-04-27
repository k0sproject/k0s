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

package config

import (
	"os"

	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display dynamic configuration reconciliation status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			os.Args = []string{os.Args[0], "kubectl", "--data-dir", opts.K0sVars.DataDir, "-n", "kube-system", "get", "event", "--field-selector", "involvedObject.name=k0s"}
			if outputFormat != "" {
				os.Args = append(os.Args, "-o", outputFormat)
			}
			return cmd.Execute()
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetKubeCtlFlagSet())
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format. Must be one of yaml|json")
	return cmd
}
