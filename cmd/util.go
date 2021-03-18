/*
Copyright 2021 Mirantis, Inc.

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

package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"
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

func cmdFlagsToArgs(cmd *cobra.Command) []string {
	flagsAndVals := []string{}
	// Use visitor to collect all flags and vals into slice
	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Value.Type() {
		case "stringSlice", "stringToString":
			val := f.Value.String()
			flagsAndVals = append(flagsAndVals, fmt.Sprintf(`--%s="%s"`, f.Name, strings.Trim(val, "[]")))
		default:
			flagsAndVals = append(flagsAndVals, fmt.Sprintf("--%s=%s", f.Name, f.Value.String()))
		}
	})
	return flagsAndVals
}

func createKubeletBootstrapConfig(clusterConfig *config.ClusterConfig, role string, expiry time.Duration) (string, error) {
	caCert, err := ioutil.ReadFile(filepath.Join(k0sVars.CertRootDir, "ca.crt"))
	if err != nil {
		msg := fmt.Sprintf("failed to read cluster ca certificate from %s. is the control plane initialized on this node?", filepath.Join(k0sVars.CertRootDir, "ca.crt"))
		return "", errors.Wrapf(err, msg)
	}
	manager, err := token.NewManager(filepath.Join(k0sVars.AdminKubeConfigPath))
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
	if role == "worker" {
		data.User = "kubelet-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.APIAddressURL()
	} else {
		data.User = "controller-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.K0sControlPlaneAPIAddress()
	}

	var buf bytes.Buffer

	err = kubeconfigTemplate.Execute(&buf, &data)
	if err != nil {
		return "", err
	}
	return token.JoinEncode(&buf)
}
