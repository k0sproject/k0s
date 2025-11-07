// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewValidateCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate k0s configuration",
		Long: `Example:
   k0s config validate --config path_to_config.yaml`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			debugFlags.Run(cmd, args)
			return internal.CallParentPersistentPreRun(cmd, args)
		},
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

	pflags := cmd.PersistentFlags()
	debugFlags.AddToFlagSet(pflags)
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
		pflags.AddFlag(f)
	})

	cmd.Flags().AddFlagSet(config.FileInputFlag())
	_ = cmd.MarkFlagRequired("config")

	return cmd
}
