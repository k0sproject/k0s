// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display dynamic configuration reconciliation status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			args := []string{"-n", "kube-system", "get", "event", "--field-selector", "involvedObject.name=k0s"}
			if outputFormat != "" {
				args = append(args, "-o", outputFormat)
			}

			return reExecKubectl(cmd, args...)
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetKubeCtlFlagSet())
	flags.StringVarP(&outputFormat, "output", "o", "", "Output format. Must be one of yaml|json")

	return cmd
}
