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
)

const (
	RoleController = "controller"
	RoleWorker     = "worker"
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

func GenerateKubeconfig(joinURL string, caCert []byte, userName string, token string) ([]byte, error) {
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
			Token: token,
		}},
	})
	return kubeconfig, err
}

func loadUserAndJoinURL(api *v1beta1.APISpec, role string) (string, string, error) {
	switch role {
	case RoleController:
		return "controller-bootstrap", api.K0sControlPlaneAPIAddress(), nil
	case RoleWorker:
		return "kubelet-bootstrap", api.APIAddressURL(), nil
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

func loadToken(ctx context.Context, k0sVars *config.CfgVars, role string, expiry time.Duration) (string, error) {
	manager, err := NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
	if err != nil {
		return "", err
	}
	return manager.Create(ctx, expiry, role)
}
