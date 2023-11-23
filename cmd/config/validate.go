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

package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
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
			var reader io.Reader

			// config.CfgFile is the global value holder for --config flag, set by cobra/pflag
			switch config.CfgFile {
			case "-":
				reader = cmd.InOrStdin()
			case "":
				return errors.New("--config can't be empty")
			default:
				f, err := os.Open(config.CfgFile)
				if err != nil {
					return err
				}
				defer f.Close()
				reader = f
			}

			cfg, err := v1beta1.ConfigFromReader(reader)
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}

			return errors.Join(cfg.Validate()...)
		},
	}

	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.Flags().AddFlagSet(config.FileInputFlag())
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
