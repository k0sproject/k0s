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
	"github.com/sirupsen/logrus"
	"html/template"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/token"
)

// TokenCommand creates new token management command
func TokenCommand() *cli.Command {
	return &cli.Command{
		Name:  "token",
		Usage: "Manage join tokens",
		Subcommands: []*cli.Command{
			CreateCommand(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:      "kubeconfig",
				Usage:     "path to kubeconfig",
				Value:     constant.AdminKubeconfigConfigPath,
				EnvVars:   []string{"KUBECONFIG"},
				TakesFile: true,
			},
		},
	}
}

var (
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
apiVersion: v1
clusters:
- cluster:
    server: {{.JoinURL}}
    certificate-authority-data: {{.CACert}}
  name: mke
contexts:
- context:
    cluster: mke
    user: {{.User}}
  name: mke
current-context: mke
kind: Config
preferences: {}
users:
- name: {{.User}}
  user:
    token: {{.Token}}
`))
)

// CreateCommand creates new command to create join tokens
func CreateCommand() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create join token",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "expiry",
				Usage: "set duration time for token",
				Value: "0",
			},
			&cli.StringFlag{
				Name:  "role",
				Usage: "Either worker or controller",
				Value: "worker",
			},
			&cli.StringFlag{
				Name:      "config",
				Value:     "mke.yaml",
				TakesFile: true,
			},
			&cli.BoolFlag{
				Name:  "wait",
				Value: false,
			},
		},
		Action: func(c *cli.Context) error {
			// Disable logrus for token commands
			logrus.SetLevel(logrus.FatalLevel)

			clusterConfig := ConfigFromYaml(c)
			expiry, err := time.ParseDuration(c.String("expiry"))
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
				return c.Bool("wait")
			}, func() error {
				bootstrapConfig, err = createKubeletBootstrapConfig(clusterConfig, c.String("role"), expiry)

				return err
			})
			if err != nil {
				return err
			}

			fmt.Println(bootstrapConfig)

			return nil
		},
	}
}

func createKubeletBootstrapConfig(clusterConfig *config.ClusterConfig, role string, expiry time.Duration) (string, error) {
	caCert, err := ioutil.ReadFile(path.Join(constant.CertRootDir, "ca.crt"))
	if err != nil {
		return "", errors.Wrapf(err, "failed to read cluster ca certificate, is the control plane initialized on this node?")
	}
	manager, err := token.NewManager(path.Join(constant.AdminKubeconfigConfigPath))
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
