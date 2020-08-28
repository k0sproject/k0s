package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/token"
)

// TokenCommand ...
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
				Value:     "/var/lib/mke/pki/admin.conf",
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
		},
		Action: func(c *cli.Context) error {

			clusterConfig, err := config.FromYaml(c.String("config"))
			if err != nil {
				logrus.Errorf("Failed to read cluster config: %s", err.Error())
				logrus.Error("THINGS MIGHT NOT WORK PROPERLY AS WE'RE GONNA USE DEFAULTS")
				clusterConfig = &config.ClusterConfig{
					Spec: config.DefaultClusterSpec(),
				}
			}
			m, err := token.NewManager(c.String("kubeconfig"))
			if err != nil {
				return err
			}
			expiry, err := time.ParseDuration(c.String("expiry"))
			if err != nil {
				return err
			}
			token, err := m.Create(expiry, c.String("role"))
			if err != nil {
				return err
			}

			caCert, err := ioutil.ReadFile("/var/lib/mke/pki/ca.crt")
			if err != nil {
				return errors.Wrapf(err, "failed to read cluster ca certificate, is the control plane initialized on this node?")
			}
			data := struct {
				CACert  string
				Token   string
				User    string
				JoinURL string
			}{
				CACert: base64.StdEncoding.EncodeToString(caCert),
				Token:  token,
			}

			if c.String("role") == "worker" {
				data.User = "kubelet-bootstrap"
				data.JoinURL = clusterConfig.Spec.API.APIAddress()
			} else {
				data.User = "controller-bootstrap"
				data.JoinURL = clusterConfig.Spec.API.ControllerJoinAddress()
			}

			var buf bytes.Buffer

			err = kubeconfigTemplate.Execute(&buf, &data)
			if err != nil {
				return err
			}

			fmt.Println(base64.StdEncoding.EncodeToString(buf.Bytes()))

			return nil
		},
	}
}
