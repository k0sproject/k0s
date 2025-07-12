// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"tool/cmd/aws/ha"
	"tool/cmd/aws/havpc"
	"tool/cmd/aws/vpcinfra"

	"github.com/spf13/cobra"
)

// NewCommand creates a cobra.Command for all AWS sub-commands.
func NewCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "aws",
		Short: "AWS specific commands for k0s installations",
	}

	cmd.AddCommand(vpcinfra.NewCommand())
	cmd.AddCommand(ha.NewCommand())
	cmd.AddCommand(havpc.NewCommand())

	return &cmd
}
