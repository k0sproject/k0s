// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/k0sproject/k0s/pkg/featuregate"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewConfigCmd() *cobra.Command {
	var featureGates featuregate.FeatureGates

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration related sub-commands",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("feature-gates") {
				return featureGates.Set("")
			}
			return nil
		},
	}

	cmd.PersistentFlags().Var(&featureGates, "feature-gates", "feature gates to enable (comma separated list of key=value pairs)")

	cmd.AddCommand(NewCreateCmd())
	cmd.AddCommand(NewEditCmd())
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewValidateCmd())

	return cmd
}

func reExecKubectl(cmd *cobra.Command, kubectlArgs ...string) error {
	args := []string{"kubectl"}
	if dataDir, err := cmd.Flags().GetString("data-dir"); err != nil {
		return err
	} else if dataDir != "" {
		args = append(args, "--data-dir", dataDir)
	}

	root := cmd.Root()
	root.SetArgs(append(args, kubectlArgs...))

	silenceErrors := root.SilenceErrors
	defer func() { root.SilenceErrors = silenceErrors }()
	root.SilenceErrors = true // So that errors aren't printed twice.
	return root.Execute()
}
