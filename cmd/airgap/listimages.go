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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/airgap"
	"github.com/k0sproject/k0s/pkg/config"
)

type CmdOpts config.CLIOptions

func NewAirgapListImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "List image names and version needed for air-gap install",
		Example: `k0s airgap list-images`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// we don't need warning messages in case of default config
			logrus.SetLevel(logrus.ErrorLevel)

			// get k0s config
			loadingRules := config.ClientConfigLoadingRules{}
			cfg, err := loadingRules.Load()
			if err != nil {
				return err
			}
			uris := airgap.GetImageURIs(cfg.Spec.Images)
			for _, uri := range uris {
				fmt.Println(uri)
			}
			return nil
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
