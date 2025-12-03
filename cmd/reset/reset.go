// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/uninstall"

	"github.com/spf13/cobra"
)

func NewResetCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:              "reset",
		Short:            "Uninstall k0s. Must be run with elevated privileges",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			return uninstall.Run(uninstall.Options{
				Vars:      opts.K0sVars,
				CriSocket: opts.CriSocket,
				Debug:     debugFlags.IsDebug(),
			})
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetCriSocketFlag())
	flags.AddFlagSet(config.FileInputFlag())
	flags.String("kubelet-root-dir", "", "Kubelet root directory for k0s")

	return cmd
}
