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

package worker

import (
	"fmt"
	"os"
	"path"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/token"
)

func HandleKubeletBootstrapToken(encodedToken string, k0sVars constant.CfgVars) error {
	kubeconfig, err := token.DecodeJoinToken(encodedToken)
	if err != nil {
		return fmt.Errorf("failed to decode token: %w", err)
	}

	// Load the bootstrap kubeconfig to validate it
	clientCfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to parse kubelet bootstrap auth from token: %w", err)
	}

	if tokenType := token.GetTokenType(clientCfg); tokenType != "kubelet-bootstrap" {
		return fmt.Errorf("wrong token type %s, expected type: kubelet-bootstrap", tokenType)
	}

	kubeletCAPath := path.Join(k0sVars.CertRootDir, "ca.crt")
	if !file.Exists(kubeletCAPath) {
		if err := dir.Init(k0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
			return fmt.Errorf("failed to initialize directory '%s': %w", k0sVars.CertRootDir, err)
		}
		err = os.WriteFile(kubeletCAPath, clientCfg.Clusters["k0s"].CertificateAuthorityData, constant.CertMode)
		if err != nil {
			return fmt.Errorf("failed to write ca client cert: %w", err)
		}
	}
	err = os.WriteFile(k0sVars.KubeletBootstrapConfigPath, kubeconfig, constant.CertSecureMode)
	if err != nil {
		return fmt.Errorf("failed writing kubelet bootstrap auth config: %w", err)
	}

	return nil
}

func LoadKubeletConfigClient(k0svars constant.CfgVars) (*KubeletConfigClient, error) {
	var kubeletConfigClient *KubeletConfigClient
	// Prefer to load client config from kubelet auth, fallback to bootstrap token auth
	clientConfigPath := k0svars.KubeletBootstrapConfigPath
	if file.Exists(k0svars.KubeletAuthConfigPath) {
		clientConfigPath = k0svars.KubeletAuthConfigPath
	}

	kubeletConfigClient, err := NewKubeletConfigClient(clientConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start kubelet config client: %v", err)
	}
	return kubeletConfigClient, nil
}
