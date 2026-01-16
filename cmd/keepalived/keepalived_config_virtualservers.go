// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"github.com/k0sproject/k0s/pkg/component/controller/cplb"
	"github.com/spf13/cobra"
)

func NewKeepalivedConfigVirtualServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "virtualservers",
		Short: "Output the default Keepalived Virtual Servers configuration template to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write([]byte(cplb.KeepalivedVirtualServersConfigTemplate))
			return err
		},
	}

	return cmd
}
