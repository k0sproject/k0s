// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewKeepalivedConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keepalived-config",
		Short: "Keepalived configuration related sub-commands",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	cmd.AddCommand(NewKeepAlivedConfigVRRPCmd())
	cmd.AddCommand(NewKeepalivedConfigVirtualServersCmd())

	return cmd
}
