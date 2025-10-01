// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"fmt"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func newAirgapListImagesCmd() *cobra.Command {
	var (
		debugFlags internal.DebugFlags
		all        bool
	)

	cmd := &cobra.Command{
		Use:              "list-images",
		Short:            "List image names and versions needed for airgapped installations",
		Example:          `k0s airgap list-images`,
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			clusterConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			out := cmd.OutOrStdout()
			for _, uri := range airgap.GetImageURIs(clusterConfig.Spec, all) {
				if _, err := fmt.Fprintln(out, uri); err != nil {
					return err
				}
			}
			return nil
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.FileInputFlag())
	flags.BoolVar(&all, "all", false, "include all images, even if they are not used in the current configuration")

	return cmd
}
