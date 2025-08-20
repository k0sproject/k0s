// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/spf13/cobra"
)

func kubeconfigCreateCmd() *cobra.Command {
	var (
		groups                  string
		certificateExpiresAfter time.Duration
		contextName             string
	)

	cmd := &cobra.Command{
		Use:   "create username",
		Short: "Create a kubeconfig for a user",
		Long: `Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user`,
		Example: `	Command to create a kubeconfig for a user:
	CLI argument:
	$ k0s kubeconfig create username

	optionally add groups:
	$ k0s kubeconfig create username --groups [groups]

	customize the expiration duration of the certificate:
	$ k0s kubeconfig create username --certificate-expires-after 8760h

	set custom context name:
	$ k0s kubeconfig create username --context-name my-cluster`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if username == "" {
				return errors.New("username cannot be empty")
			}

			if contextName == "" {
				return errors.New("context-name cannot be empty")
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

			kubeconfig, err := createUserKubeconfig(opts.K0sVars, clusterAPIURL, username, groups, certificateExpiresAfter, contextName)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(kubeconfig)
			return err
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.FileInputFlag())
	flags.StringVar(&groups, "groups", "", "Specify groups")
	flags.DurationVar(&certificateExpiresAfter, "certificate-expires-after", 8760*time.Hour, "The expiration duration of the certificate")
	flags.StringVar(&contextName, "context-name", "k0s", "Specify kubeconfig context name")
	return cmd
}

func createUserKubeconfig(k0sVars *config.CfgVars, clusterAPIURL, username, groups string, certificateExpiresAfter time.Duration, contextName string) ([]byte, error) {
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
	userCert, err := certManager.EnsureCertificate(userReq, users.RootUID, certificateExpiresAfter)
	if err != nil {
		return nil, fmt.Errorf("failed generate user certificate: %w, check if the control plane is initialized on this node", err)
	}

	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{contextName: {
			Server:               clusterAPIURL,
			CertificateAuthority: userReq.CACert,
		}},
		Contexts: map[string]*clientcmdapi.Context{contextName: {
			Cluster:  contextName,
			AuthInfo: username,
		}},
		CurrentContext: contextName,
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
