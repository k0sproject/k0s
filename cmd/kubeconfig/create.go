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
package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path"

	"github.com/cloudflare/cfssl/log"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/config"
)

var (
	groups string

	userKubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
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
    client-certificate-data: {{.ClientCert}}
    client-key-data: {{.ClientKey}}
`))
)

func kubeconfigCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [username]",
		Short: "Create a kubeconfig for a user",
		Long: `Create a kubeconfig with a signed certificate and public key for a given user (and optionally user groups)
Note: A certificate once signed cannot be revoked for a particular user`,
		Example: `	Command to create a kubeconfig for a user:
	CLI argument:
	$ k0s kubeconfig create [username]

	optionally add groups:
	$ k0s kubeconfig create [username] --groups [groups]`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// disable cfssl log
			log.Level = log.LevelFatal

			if len(args) == 0 {
				return fmt.Errorf("username is mandatory")
			}
			username := args[0]
			c := CmdOpts(config.GetCmdOpts())
			// get k0s config
			loadingRules := config.ClientConfigLoadingRules{}
			cfg, err := loadingRules.Load()
			if err != nil {
				return err
			}

			clusterAPIURL := cfg.Spec.API.APIAddressURL()
			caCert, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "ca.crt"))
			if err != nil {
				return fmt.Errorf("failed to read cluster ca certificate: %w, check if the control plane is initialized on this node", err)
			}
			caCertPath, caCertKey := path.Join(c.K0sVars.CertRootDir, "ca.crt"), path.Join(c.K0sVars.CertRootDir, "ca.key")
			userReq := certificate.Request{
				Name:   username,
				CN:     username,
				O:      groups,
				CACert: caCertPath,
				CAKey:  caCertKey,
			}
			certManager := certificate.Manager{
				K0sVars: c.K0sVars,
			}
			userCert, err := certManager.EnsureCertificate(userReq, "root")
			if err != nil {
				return err
			}

			data := struct {
				CACert     string
				ClientCert string
				ClientKey  string
				User       string
				JoinURL    string
			}{
				CACert:     base64.StdEncoding.EncodeToString(caCert),
				ClientCert: base64.StdEncoding.EncodeToString([]byte(userCert.Cert)),
				ClientKey:  base64.StdEncoding.EncodeToString([]byte(userCert.Key)),
				User:       username,
				JoinURL:    clusterAPIURL,
			}

			var buf bytes.Buffer

			err = userKubeconfigTemplate.Execute(&buf, &data)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(buf.Bytes())
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&groups, "groups", "", "Specify groups")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
