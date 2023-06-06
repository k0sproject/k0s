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
	"fmt"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewAirgapListImagesCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "List image names and version needed for air-gap install",
		Example: `k0s airgap list-images`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			if opts.EnableDynamicConfig {
				return fmt.Errorf("dynamic config is not supported for airgap list-images")
			}

			clusterConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			for _, uri := range airgap.GetImageURIs(clusterConfig.Spec, all) {
				fmt.Fprintln(cmd.OutOrStdout(), uri)
			}
			return nil
		},
	}
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	cmd.Flags().BoolVar(&all, "all", false, "include all images, even if they are not used in the current configuration")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
