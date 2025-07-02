// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/client-go/tools/clientcmd"

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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			// The admin kubeconfig in k0s' data dir uses the internal cluster
			// address. This command is intended to provide a kubeconfig that
			// uses the external address. Load the existing admin kubeconfig and
			// rewrite it.
			adminConfig, err := kubernetes.KubeconfigFromFile(opts.K0sVars.AdminKubeConfigPath)()
			if pathErr := (*fs.PathError)(nil); errors.As(err, &pathErr) &&
				pathErr.Path == opts.K0sVars.AdminKubeConfigPath &&
				errors.Is(pathErr.Err, fs.ErrNotExist) {
				return fmt.Errorf("admin config %q not found, check if the control plane is initialized on this node", pathErr.Path)
			}
			if err != nil {
				return fmt.Errorf("failed to load admin config: %w", err)
			}

			// Now replace the internal address with the external one. See
			// cmd/controller/certificates.go to see how the original kubeconfig
			// is generated.
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			internalURL := nodeConfig.Spec.API.LocalURL().String()
			externalURL := nodeConfig.Spec.API.APIAddressURL()
			for _, c := range adminConfig.Clusters {
				if c.Server == internalURL {
					c.Server = externalURL
				}
			}

			data, err := clientcmd.Write(*adminConfig)
			if err != nil {
				return fmt.Errorf("failed to serialize admin kubeconfig: %w", err)
			}

			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}

	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())

	return cmd
}
