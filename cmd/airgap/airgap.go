/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
