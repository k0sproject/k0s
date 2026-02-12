// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package payload

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewPayloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payload",
		Short: "Manage embedded payload",
		Args:  cobra.NoArgs,
		Long:  `Commands for managing embedded payload in k0s`,
		RunE:  func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	cmd.AddCommand(NewExtractCmd())

	return cmd
}
