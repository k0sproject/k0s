// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"fmt"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/containerd/platforms"
	"github.com/spf13/cobra"
)

func newAirgapListImagesCmd() *cobra.Command {
	var (
		debugFlags internal.DebugFlags
		targetEnv  = airgap.TargetEnv{
			Platform: platforms.DefaultSpec(),
		}
		all bool
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

			if nodeConfig, err := opts.K0sVars.NodeConfig(); err != nil {
				return fmt.Errorf("failed to get node config: %w", err)
			} else {
				targetEnv.Spec = nodeConfig.Spec
			}

			out := cmd.OutOrStdout()
			for _, uri := range airgap.GetImageURIs(targetEnv, all) {
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
	flags.Var((*platformFlag)(&targetEnv.Platform), "platform", "the platform to list images for")
	flags.BoolVar(&all, "all", false, "include all images, even if they are not used in the current configuration")

	return cmd
}
