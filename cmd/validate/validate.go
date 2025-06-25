// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	configcmd "github.com/k0sproject/k0s/cmd/config"

	"github.com/spf13/cobra"
)

// TODO deprecated, remove when appropriate
func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "validate",
		Short:      "Validation related sub-commands",
		Hidden:     true,
		Deprecated: "use 'k0s config validate' instead",
	}
	cmd.AddCommand(newConfigCmd())
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := configcmd.NewValidateCmd()
	cmd.Use = "config"
	cmd.Deprecated = "use 'k0s config validate' instead"
	cmd.Hidden = false
	return cmd
}
