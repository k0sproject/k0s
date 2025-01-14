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

	"github.com/spf13/cobra"
)

func tokenInvalidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invalidate join-token...",
		Short:   "Invalidates existing join token",
		Example: "k0s token invalidate xyz123",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			manager, err := token.NewManager(opts.K0sVars.AdminKubeConfigPath)
			if err != nil {
				return err
			}

			for _, id := range args {
				err := manager.Remove(cmd.Context(), id)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "token %s deleted successfully\n", id)
			}
			return nil
		},
	}

	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())

	return cmd
}
