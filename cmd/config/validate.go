/*
Copyright 2020 k0s authors

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

package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate k0s configuration",
		Long: `Example:
   k0s config validate --config path_to_config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := config.GetCmdOpts()

			var cfgFile io.Reader
			switch c.CfgFile {
			case "":
				return fmt.Errorf("config file not specified")
			case "-":
				cfgFile = cmd.InOrStdin()
			default:
				f, err := os.Open(c.CfgFile)
				if err != nil {
					return err
				}
				defer f.Close()
				cfgFile = f
			}

			cfg, err := v1beta1.ConfigFromReader(cfgFile)
			if err != nil {
				return err
			}

			return errors.Join(cfg.Validate()...)
		},
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	return cmd
}
