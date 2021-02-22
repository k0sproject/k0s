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
package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"time"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/token"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func init() {
	tokenCreateCmd.Flags().StringVar(&tokenExpiry, "expiry", "0s", "Expiration time of the token. Format 1.5h, 2h45m or 300ms.")
	tokenCreateCmd.Flags().StringVar(&tokenRole, "role", "worker", "Either worker or controller")
	tokenCreateCmd.Flags().BoolVar(&waitCreate, "wait", false, "wait forever (default false)")

	addPersistentFlags(tokenCreateCmd)

	// shell completion options
	_ = tokenCreateCmd.MarkFlagRequired("role")
	_ = tokenCreateCmd.RegisterFlagCompletionFunc("role", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"worker", "controller"}, cobra.ShellCompDirectiveDefault
	})
}

var (
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

	tokenCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create join token",
		Example: `k0s token create --role worker --expiry 100h //sets expiration time to 100 hours
k0s token create --role worker --expiry 10m  //sets expiration time to 10 minutes
`,
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
