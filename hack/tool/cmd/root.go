// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"os"

	"tool/cmd/aws"

	"github.com/spf13/cobra"
)

func newCommandRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "tool is the tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("insufficient arguments")
		},
	}

	cmd.AddCommand(aws.NewCommand())

	return cmd
}

func Execute() {
	if err := newCommandRoot().Execute(); err != nil {
		os.Exit(1)
	}
}
