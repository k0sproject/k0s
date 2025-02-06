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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			var bytes []byte

			// config.CfgFile is the global value holder for --config flag, set by cobra/pflag
			switch config.CfgFile {
			case "-":
				if bytes, err = io.ReadAll(cmd.InOrStdin()); err != nil {
					return fmt.Errorf("failed to read configuration from standard input: %w", err)
				}
			case "":
				return errors.New("--config can't be empty")
			default:
				if bytes, err = os.ReadFile(config.CfgFile); err != nil {
					return fmt.Errorf("failed to read configuration file: %w", err)
				}
			}

			cfg, err := v1beta1.ConfigFromBytes(bytes)
			if err != nil {
				return fmt.Errorf("failed to parse configuration: %w", err)
			}

			return errors.Join(cfg.Validate()...)
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.FileInputFlag())
	_ = cmd.MarkFlagRequired("config")

	return cmd
}
