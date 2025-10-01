// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewAirgapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "airgap",
		Short: "Tooling for airgapped installations",
		Long: `Tooling for airgapped installations.

For example, to create an image bundle that contains the images required for
the current configuration, use the following command:

    k0s airgap list-images | k0s airgap bundle-artifacts -v -o image-bundle.tar
`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	var deprecatedFlags pflag.FlagSet
	(&internal.DebugFlags{}).AddToFlagSet(&deprecatedFlags)
	deprecatedFlags.AddFlagSet(config.GetPersistentFlagSet())
	deprecatedFlags.AddFlagSet(config.FileInputFlag())
	deprecatedFlags.VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
		cmd.PersistentFlags().AddFlag(f)
	})

	log := logrus.StandardLogger()
	cmd.AddCommand(newAirgapListImagesCmd())
	cmd.AddCommand(newAirgapBundleArtifactsCmd(log, nil))

	return cmd
}
