// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"fmt"

	"github.com/k0sproject/k0s/pkg/token"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage join tokens",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	cmd.AddCommand(tokenListCmd())
	cmd.AddCommand(tokenInvalidateCmd())
	cmd.AddCommand(preSharedCmd())
	addPlatformSpecificCommands(cmd)

	return cmd
}

func checkTokenRole(tokenRole string) error {
	if tokenRole != token.RoleController && tokenRole != token.RoleWorker {
		return fmt.Errorf("unsupported role %q; supported roles are %q and %q", tokenRole, token.RoleController, token.RoleWorker)
	}
	return nil
}
