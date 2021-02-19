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
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/token"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func init() {
	tokenListCmd.Flags().StringVar(&tokenRole, "role", "", "Either worker,controller or empty for all roles")
}

var (
	tokenListCmd = &cobra.Command{
		Use:     "list",
		Short:   "List join tokens",
		Example: `k0s token list --role worker // list worker tokens`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := token.NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
			if err != nil {
				return err
			}

			tokens, err := manager.List(tokenRole)
			if err != nil {
				return err
			}
			if len(tokens) == 0 {
				fmt.Println("No k0s join tokens found")
				return nil
			}

			//fmt.Printf("Tokens: %v \n", tokens)
			table := tablewriter.NewWriter(os.Stdout)
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
)
