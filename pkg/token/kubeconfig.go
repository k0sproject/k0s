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
	controllerRole = "controller"
	workerRole     = "worker"
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

func CreateKubeletBootstrapConfig(clusterConfig *v1beta1.ClusterConfig, k0sVars constant.CfgVars, role string, expiry time.Duration) (string, error) {
	crtFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")
	caCert, err := os.ReadFile(crtFile)
	if err != nil {
		return "", fmt.Errorf("failed to read cluster ca certificate from %s: %w. check if the control plane is initialized on this node", crtFile, err)
	}
	manager, err := NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
	if err != nil {
		return "", err
	}
	tokenString, err := manager.Create(expiry, role)
	if err != nil {
		return "", err
	}
	data := struct {
		CACert  string
		Token   string
		User    string
		JoinURL string
		APIUrl  string
	}{
		CACert: base64.StdEncoding.EncodeToString(caCert),
		Token:  tokenString,
	}
	if role == workerRole {
		data.User = "kubelet-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.APIAddressURL()
	} else if role == controllerRole {
		data.User = "controller-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.K0sControlPlaneAPIAddress()
	} else {
		return "", fmt.Errorf("unsupported role %s only supported roles are %q and %q", role, controllerRole, workerRole)
	}

	var buf bytes.Buffer

	err = kubeconfigTemplate.Execute(&buf, &data)
	if err != nil {
		return "", err
	}
	return JoinEncode(&buf)
}
