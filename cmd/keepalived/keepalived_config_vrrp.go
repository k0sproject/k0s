// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"github.com/k0sproject/k0s/pkg/component/controller/cplb"
	"github.com/spf13/cobra"
)

func NewKeepAlivedConfigVRRPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vrrp",
		Short: "Output the default Keepalived VRRP configuration template to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write([]byte(cplb.KeepalivedVRRPConfigTemplate))
			return err
		},
	}

	return cmd
}
