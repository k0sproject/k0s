// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Launch the editor configured in your shell to edit k0s configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return reExecKubectl(cmd, "-n", "kube-system", "edit", "clusterconfig", "k0s")
		},
	}

	cmd.Flags().AddFlagSet(config.GetKubeCtlFlagSet())

	return cmd
}
