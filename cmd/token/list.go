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
	"io"
	"text/tabwriter"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"

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

			tokens, err := manager.List(cmd.Context())
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No k0s join tokens found")
				return nil
			}

			printTokens(cmd.OutOrStdout(), tokens, listTokenRole)

			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&listTokenRole, "role", "", "Either worker, controller or empty for all roles")

	return cmd
}

func printTokens(writer io.Writer, tokens []token.Token, listTokenRole string) {
	// Create a metav1.Table object to hold the data
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "ID", Type: "string", Description: "Token ID"},
			{Name: "Role", Type: "string", Description: "Token Role"},
			{Name: "Expires at", Type: "string", Description: "Expiration Time"},
		},
	}

	// Populate the rows
	for _, t := range tokens {
		if listTokenRole == "" || listTokenRole == t.Role {
			table.Rows = append(table.Rows, metav1.TableRow{
				Cells: []any{t.ID, t.Role, t.Expiry},
			})
		}
	}

	// Use a TabWriter for output
	tabWriter := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	defer tabWriter.Flush()

	// Use the TablePrinter to render the table
	printer := printers.NewTablePrinter(printers.PrintOptions{
		WithNamespace: false,
		Wide:          false,
		ShowLabels:    false,
	})
	if err := printer.PrintObj(table, tabWriter); err != nil {
		fmt.Fprintf(writer, "Error printing table: %v\n", err)
	}
}
