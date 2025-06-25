// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
