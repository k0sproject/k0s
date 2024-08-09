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
	"errors"
	"fmt"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func kubeconfigCreateCmd() *cobra.Command {
	var groups string

	cmd := &cobra.Command{
		Use:   "create username",
		Short: "Create a kubeconfig for a user",
		Long: `Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user`,
		Example: `	Command to create a kubeconfig for a user:
	CLI argument:
	$ k0s kubeconfig create username

	optionally add groups:
	$ k0s kubeconfig create username --groups [groups]`,
		PreRun: func(cmd *cobra.Command, args []string) {
			// ensure logs don't mess up the output
			logrus.SetOutput(cmd.ErrOrStderr())
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if username == "" {
				return errors.New("username cannot be empty")
			}

			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			clusterAPIURL := nodeConfig.Spec.API.APIAddressURL()

			kubeconfig, err := createUserKubeconfig(opts.K0sVars, clusterAPIURL, username, groups)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(kubeconfig)
			return err
		},
	}

	cmd.Flags().AddFlagSet(config.FileInputFlag())
	cmd.Flags().StringVar(&groups, "groups", "", "Specify groups")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func createUserKubeconfig(k0sVars *config.CfgVars, clusterAPIURL, username, groups string) ([]byte, error) {
	userReq := certificate.Request{
		Name:   username,
		CN:     username,
		O:      groups,
		CACert: filepath.Join(k0sVars.CertRootDir, "ca.crt"),
		CAKey:  filepath.Join(k0sVars.CertRootDir, "ca.key"),
	}
	certManager := certificate.Manager{
		K0sVars: k0sVars,
	}
	userCert, err := certManager.EnsureCertificate(userReq, "root")
	if err != nil {
		return nil, fmt.Errorf("failed generate user certificate: %w, check if the control plane is initialized on this node", err)
	}

	const k0sContextName = "k0s"
	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{k0sContextName: {
			Server:               clusterAPIURL,
			CertificateAuthority: userReq.CACert,
		}},
		Contexts: map[string]*clientcmdapi.Context{k0sContextName: {
			Cluster:  k0sContextName,
			AuthInfo: username,
		}},
		CurrentContext: k0sContextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{username: {
			ClientCertificateData: []byte(userCert.Cert),
			ClientKeyData:         []byte(userCert.Key),
		}},
	}
	if err := clientcmdapi.FlattenConfig(&kubeconfig); err != nil {
		return nil, err
	}

	return clientcmd.Write(kubeconfig)
}
