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
package token

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

const (
	RoleController = "controller"
	RoleWorker     = "worker"
)

var kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
apiVersion: v1
clusters:
- cluster:
    server: {{.JoinURL}}
    certificate-authority-data: {{.CACert}}
  name: k0s
contexts:
- context:
    cluster: k0s
    user: {{.User}}
  name: k0s
current-context: k0s
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    token: {{.Token}}
`))

func CreateKubeletBootstrapConfig(ctx context.Context, api *v1beta1.APISpec, k0sVars constant.CfgVars, role string, expiry time.Duration) (string, error) {
	data := struct {
		CACert  string
		Token   string
		User    string
		JoinURL string
		APIUrl  string
	}{}

	var err error
	data.User, data.JoinURL, err = loadUserAndJoinURL(api, role)
	if err != nil {
		return "", err
	}
	data.CACert, err = loadCACert(k0sVars)
	if err != nil {
		return "", err
	}
	data.Token, err = loadToken(ctx, k0sVars, role, expiry)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = kubeconfigTemplate.Execute(&buf, &data)
	if err != nil {
		return "", err
	}
	return JoinEncode(&buf)
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

func loadCACert(k0sVars constant.CfgVars) (string, error) {
	crtFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")
	caCert, err := os.ReadFile(crtFile)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster CA from %q: %w; check if the control plane is initialized on this node", crtFile, err)
	}

	return base64.StdEncoding.EncodeToString(caCert), nil
}

func loadToken(ctx context.Context, k0sVars constant.CfgVars, role string, expiry time.Duration) (string, error) {
	manager, err := NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
	if err != nil {
		return "", err
	}
	return manager.Create(ctx, expiry, role)
}
