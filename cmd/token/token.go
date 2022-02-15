/*
Copyright 2022 k0s authors

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
package token

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"
)

type CmdOpts config.CLIOptions

var (
	tokenExpiry string
	waitCreate  bool
)

func NewTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage join tokens",
	}

	cmd.SilenceUsage = true
	cmd.AddCommand(tokenCreateCmd())
	cmd.AddCommand(tokenListCmd())
	cmd.AddCommand(tokenInvalidateCmd())
	return cmd
}

func checkTokenRole(tokenRole string) error {
	if tokenRole != token.RoleController && tokenRole != token.RoleWorker {
		return fmt.Errorf("unsupported role %q; supported roles are %q and %q", tokenRole, token.RoleController, token.RoleWorker)
	}
	return nil
}
