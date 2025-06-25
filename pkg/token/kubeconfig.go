// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
)

const (
	RoleController = "controller"
	RoleWorker     = "worker"
)

const (
	ControllerTokenAuthName = "controller-bootstrap"
	WorkerTokenAuthName     = "kubelet-bootstrap"
)

// CreateKubeletBootstrapToken creates a new k0s bootstrap token.
func CreateKubeletBootstrapToken(ctx context.Context, api *v1beta1.APISpec, k0sVars *config.CfgVars, role string, expiry time.Duration) (string, error) {
	userName, joinURL, err := loadUserAndJoinURL(api, role)
	if err != nil {
		return "", err
	}

	caCert, err := loadCACert(k0sVars)
	if err != nil {
		return "", err
	}

	token, err := loadToken(ctx, k0sVars, role, expiry)
	if err != nil {
		return "", err
	}

	kubeconfig, err := GenerateKubeconfig(joinURL, caCert, userName, token)
	if err != nil {
		return "", err
	}

	return JoinEncode(bytes.NewReader(kubeconfig))
}

func GenerateKubeconfig(joinURL string, caCert []byte, userName string, token *bootstraptokenv1.BootstrapTokenString) ([]byte, error) {
	const k0sContextName = "k0s"
	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{k0sContextName: {
			Server:                   joinURL,
			CertificateAuthorityData: caCert,
		}},
		Contexts: map[string]*clientcmdapi.Context{k0sContextName: {
			Cluster:  k0sContextName,
			AuthInfo: userName,
		}},
		CurrentContext: k0sContextName,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{userName: {
			Token: token.String(),
		}},
	})
	return kubeconfig, err
}

func loadUserAndJoinURL(api *v1beta1.APISpec, role string) (string, string, error) {
	switch role {
	case RoleController:
		return ControllerTokenAuthName, api.K0sControlPlaneAPIAddress(), nil
	case RoleWorker:
		return WorkerTokenAuthName, api.APIAddressURL(), nil
	default:
		return "", "", fmt.Errorf("unsupported role %q; supported roles are %q and %q", role, RoleController, RoleWorker)
	}
}

func loadCACert(k0sVars *config.CfgVars) ([]byte, error) {
	crtFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")
	caCert, err := os.ReadFile(crtFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster CA from %q: %w; check if the control plane is initialized on this node", crtFile, err)
	}

	return caCert, nil
}

func loadToken(ctx context.Context, k0sVars *config.CfgVars, role string, expiry time.Duration) (*bootstraptokenv1.BootstrapTokenString, error) {
	manager, err := NewManager(k0sVars.AdminKubeConfigPath)
	if err != nil {
		return nil, err
	}
	return manager.Create(ctx, expiry, role)
}
