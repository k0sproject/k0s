// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

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

			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}

			const (
				clusterName = "local"
				contextName = "Default"
				userName    = "user"
			)

			adminConfig := clientcmdapi.Config{
				Clusters: map[string]*clientcmdapi.Cluster{clusterName: {
					Server:               nodeConfig.Spec.API.APIAddressURL(),
					CertificateAuthority: filepath.Join(opts.K0sVars.CertRootDir, "ca.crt"),
				}},
				Contexts: map[string]*clientcmdapi.Context{contextName: {
					Cluster:  clusterName,
					AuthInfo: userName,
				}},
				CurrentContext: contextName,
				AuthInfos: map[string]*clientcmdapi.AuthInfo{userName: {
					ClientCertificate: filepath.Join(opts.K0sVars.CertRootDir, "admin.crt"),
					ClientKey:         filepath.Join(opts.K0sVars.CertRootDir, "admin.key"),
				}},
			}

			if err := clientcmdapi.FlattenConfig(&adminConfig); err != nil {
				if pathErr := (*fs.PathError)(nil); errors.As(err, &pathErr) &&
					filepath.Dir(pathErr.Path) == opts.K0sVars.CertRootDir &&
					errors.Is(pathErr.Err, fs.ErrNotExist) {
					return fmt.Errorf("admin PKI file %q not found, check if the control plane is initialized on this node", pathErr.Path)
				}
				return fmt.Errorf("failed to create admin kubeconfig: %w", err)
			}

			data, err := clientcmd.Write(adminConfig)
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
