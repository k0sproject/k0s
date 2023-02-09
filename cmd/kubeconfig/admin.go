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

package kubeconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
)

func kubeConfigAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Display Admin's Kubeconfig file",
		Long:  "Print kubeconfig for the Admin user to stdout",
		Example: `	$ k0s kubeconfig admin > ~/.kube/config
	$ export KUBECONFIG=~/.kube/config
	$ kubectl get nodes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := config.GetCmdOpts()
			content, err := os.ReadFile(c.K0sVars.AdminKubeConfigPath)
			if err != nil {
				return fmt.Errorf("failed to read admin config, check if the control plane is initialized on this node: %w", err)
			}

			clusterAPIURL := c.NodeConfig.Spec.API.APIAddressURL()
			newContent := strings.Replace(string(content), "https://localhost:6443", clusterAPIURL, -1)
			_, err = cmd.OutOrStdout().Write([]byte(newContent))
			return err
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
