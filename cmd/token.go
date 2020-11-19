/*
Copyright 2020 Mirantis, Inc.

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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"
)

func init() {
	tokenCmd.Flags().StringVar(&kubeConfig, "kubeconfig", k0sVars.AdminKubeConfigPath, "path to kubeconfig file [$KUBECONFIG]")
	if kubeConfig == "" {
		kubeConfig = viper.GetString("KUBECONFIG")
	}
	tokenCreateCmd.Flags().StringVar(&tokenExpiry, "expiry", "0", "set duration time for token")
	tokenCreateCmd.Flags().StringVar(&tokenRole, "role", "worker", "Either worker or controller")
	tokenCreateCmd.Flags().BoolVar(&waitCreate, "wait", false, "wait forever (default false)")

	tokenCmd.AddCommand(tokenCreateCmd)
}

var (
	kubeConfig  string
	tokenExpiry string
	tokenRole   string
	waitCreate  bool

	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
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

	// tokenCmd creates new token management command
	tokenCmd = &cobra.Command{
		Use:   "token",
		Short: "Manage join tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tokenCreateCmd.Usage()
		},
	}

	tokenCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create join token",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable logrus for token commands
			logrus.SetLevel(logrus.FatalLevel)

			clusterConfig, err := ConfigFromYaml(cfgFile)
			if err != nil {
				return err
			}
			expiry, err := time.ParseDuration(tokenExpiry)
			if err != nil {
				return err
			}

			var bootstrapConfig string
			// we will retry every second for two minutes and then error
			err = retry.OnError(wait.Backoff{
				Steps:    120,
				Duration: 1 * time.Second,
				Factor:   1.0,
				Jitter:   0.1,
			}, func(err error) bool {
				return waitCreate
			}, func() error {
				bootstrapConfig, err = createKubeletBootstrapConfig(clusterConfig, tokenRole, expiry)

				return err
			})
			if err != nil {
				return err
			}

			fmt.Println(bootstrapConfig)

			return nil
		},
	}
)

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
	}{
		CACert: base64.StdEncoding.EncodeToString(caCert),
		Token:  tokenString,
	}
	if role == "worker" {
		data.User = "kubelet-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.APIAddress()
	} else {
		data.User = "controller-bootstrap"
		data.JoinURL = clusterConfig.Spec.API.ControllerJoinAddress()
	}

	var buf bytes.Buffer

	err = kubeconfigTemplate.Execute(&buf, &data)
	if err != nil {
		return "", err
	}
	return token.JoinEncode(&buf)
}
