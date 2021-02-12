/*
Copyright 2020 Mirantis, Inc.

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
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/token"
	"github.com/spf13/cobra"
)

func init() {

}

var (
	tokenInvalidateCmd = &cobra.Command{
		Use:     "invalidate",
		Short:   "Invalidates existing join token",
		Example: "k0s token invalidate xyz123",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("invalidate requires at least one token Id to invalidate")
			}
			manager, err := token.NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
			if err != nil {
				return err
			}

			for _, id := range args {
				err := manager.Remove(id)
				if err != nil {
					return err
				}
				fmt.Printf("token %s deleted succesfully\n", id)
			}
			return nil
		},
	}
)
