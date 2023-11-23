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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"
)

func preSharedCmd() *cobra.Command {
	var (
		certPath      string
		joinURL       string
		preSharedRole string
		outDir        string
		validity      time.Duration
	)

	cmd := &cobra.Command{
		Use:     "pre-shared",
		Short:   "Generates token and secret and stores them as a files",
		Example: `k0s token pre-shared --role worker --cert <path>/<to>/ca.crt --url https://<controller-ip>:<port>/`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			err := checkTokenRole(preSharedRole)
			if err != nil {
				return err
			}
			if certPath == "" {
				return fmt.Errorf("please, provide --cert argument")
			}
			if joinURL == "" {
				return fmt.Errorf("please, provide --url argument")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := createSecret(preSharedRole, validity, outDir)
			if err != nil {
				return err
			}

			err = createKubeConfig(t, preSharedRole, joinURL, certPath, outDir)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&certPath, "cert", "", "path to the CA certificate file")
	cmd.Flags().StringVar(&joinURL, "url", "", "url of the api server to join")
	cmd.Flags().StringVar(&preSharedRole, "role", "worker", "token role. valid values: worker, controller. Default: worker")
	cmd.Flags().StringVar(&outDir, "out", ".", "path to the output directory. Default: current dir")
	cmd.Flags().DurationVar(&validity, "valid", 0, "how long token is valid, in Go duration format")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func createSecret(role string, validity time.Duration, outDir string) (string, error) {
	fakeClient := fakeclientset.NewSimpleClientset()

	manager, err := token.NewManagerForClient(fakeClient)
	if err != nil {
		return "", fmt.Errorf("error creating token manager: %w", err)
	}

	t, err := manager.Create(context.Background(), validity, role)
	if err != nil {
		return "", fmt.Errorf("error creating token: %w", err)
	}

	// Get created Secret from the fake client and write it as a file
	for _, action := range fakeClient.Actions() {
		a, ok := action.(testing.CreateActionImpl)
		if !ok {
			continue
		}

		secret, ok := a.GetObject().(*v1.Secret)
		if !ok {
			continue
		}
		secret.APIVersion = "v1"
		secret.Kind = "Secret"

		b, err := yaml.Marshal(secret)
		if err != nil {
			return "", fmt.Errorf("error marshailling secret: %w", err)
		}

		err = file.WriteContentAtomically(filepath.Join(outDir, secret.Name+".yaml"), b, 0640)
		if err != nil {
			return "", fmt.Errorf("error writing secret: %w", err)
		}
	}
	return t, nil
}

func createKubeConfig(tokenString, role, joinURL, certPath, outDir string) error {
	caCert, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("error reading certificate: %w", err)
	}

	var userName string
	switch role {
	case "worker":
		userName = "kubelet-bootstrap"
	case "controller":
		userName = "controller-bootstrap"
	default:
		return fmt.Errorf("unknown role: %s", role)
	}
	kubeconfig, err := token.GenerateKubeconfig(joinURL, caCert, userName, tokenString)
	if err != nil {
		return fmt.Errorf("error generating kubeconfig: %w", err)
	}

	encodedToken, err := token.JoinEncode(bytes.NewReader(kubeconfig))
	if err != nil {
		return fmt.Errorf("error encoding token: %w", err)
	}

	err = file.WriteContentAtomically(filepath.Join(outDir, "token_"+tokenString), []byte(encodedToken), 0640)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig: %w", err)
	}

	return nil
}
