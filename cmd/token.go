package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

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
    server: https://127.0.0.1:6443
    certificate-authority-data: {{.CACert}}
  name: mke
contexts:
- context:
    cluster: mke
    user: kubelet-bootstrap
  name: mke
current-context: mke
kind: Config
preferences: {}
users:
- name: kubelet-bootstrap
  user:
    token: {{.Token}}
`))
)

func CreateCommand() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create join token",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			m, err := token.NewManager(c.String("kubeconfig"))
			if err != nil {
				return err
			}
			token, err := m.Create()
			if err != nil {
				return err
			}

			caCert, err := ioutil.ReadFile("/var/lib/mke/pki/ca.crt")
			if err != nil {
				return errors.Wrapf(err, "failed to read cluster ca certificate, is the control plane initialized on this node?")
			}
			data := struct {
				CACert string
				Token  string
			}{
				CACert: base64.StdEncoding.EncodeToString(caCert),
				Token:  token,
			}

			var buf bytes.Buffer

			err = kubeconfigTemplate.Execute(&buf, &data)
			if err != nil {
				return err
			}

			// kubeconfig := buf.String()

			fmt.Println(base64.StdEncoding.EncodeToString(buf.Bytes()))

			return nil
		},
	}
}
