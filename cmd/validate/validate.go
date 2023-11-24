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

package validate

import (
	configcmd "github.com/k0sproject/k0s/cmd/config"

	"github.com/spf13/cobra"
)

// TODO deprecated, remove when appropriate
func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "validate",
		Short:  "Validation related sub-commands",
		Hidden: true,
	}
	cmd.AddCommand(newConfigCmd())
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := configcmd.NewValidateCmd()
	cmd.Use = "config"
	cmd.Deprecated = "use 'k0s config validate' instead"
	cmd.Hidden = false
	return cmd
}
