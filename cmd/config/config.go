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
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration related sub-commands",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}
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
