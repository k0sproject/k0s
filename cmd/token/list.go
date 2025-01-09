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

package token

import (
	"fmt"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func tokenListCmd() *cobra.Command {
	var listTokenRole string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List join tokens",
		Example: `k0s token list --role worker // list worker tokens`,
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return checkTokenRole(listTokenRole)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			manager, err := token.NewManager(opts.K0sVars.AdminKubeConfigPath)
			if err != nil {
				return err
			}

			tokens, err := manager.List(cmd.Context(), listTokenRole)
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No k0s join tokens found")
				return nil
			}

			table := tablewriter.NewWriter(cmd.OutOrStdout())
			table.SetHeader([]string{"ID", "Role", "Expires at"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(true)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(false)
			table.SetTablePadding("\t") // pad with tabs
			table.SetNoWhiteSpace(true)
			for _, t := range tokens {
				table.Append(t.ToArray())
			}

			table.Render()

			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&listTokenRole, "role", "", "Either worker, controller or empty for all roles")

	return cmd
}
