// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/token"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := checkTokenRole(preSharedRole); err != nil {
				return err
			}

			t, err := createSecret(preSharedRole, validity, outDir)
			if err != nil {
				return err
			}

			return createKubeConfig(t, preSharedRole, joinURL, certPath, outDir)
		},
	}

	var deprecatedFlags pflag.FlagSet
	(&internal.DebugFlags{}).AddToFlagSet(&deprecatedFlags)
	deprecatedFlags.AddFlagSet(config.GetPersistentFlagSet())
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		cmd.PersistentFlags().AddFlag(f)
	})

	flags := cmd.Flags()
	flags.StringVar(&certPath, "cert", "", "path to the CA certificate file")
	runtime.Must(cobra.MarkFlagRequired(flags, "cert"))
	flags.StringVar(&joinURL, "url", "", "url of the api server to join")
	runtime.Must(cobra.MarkFlagRequired(flags, "url"))
	flags.StringVar(&preSharedRole, "role", "worker", "token role. valid values: worker, controller. Default: worker")
	flags.StringVar(&outDir, "out", ".", "path to the output directory. Default: current dir")
	flags.DurationVar(&validity, "valid", 0, "how long token is valid, in Go duration format")

	return cmd
}

func createSecret(role string, validity time.Duration, outDir string) (*bootstraptokenv1.BootstrapTokenString, error) {
	secret, token, err := token.RandomBootstrapSecret(role, validity)
	if err != nil {
		return nil, fmt.Errorf("failed to generate bootstrap secret: %w", err)
	}

	if err := file.WriteAtomically(filepath.Join(outDir, secret.Name+".yaml"), 0640, func(unbuffered io.Writer) error {
		serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		encoder := scheme.Codecs.EncoderForVersion(serializer, corev1.SchemeGroupVersion)
		w := bufio.NewWriter(unbuffered)
		if err := encoder.Encode(secret, w); err != nil {
			return err
		}
		return w.Flush()
	}); err != nil {
		return nil, fmt.Errorf("failed to save bootstrap secret: %w", err)
	}

	return token, nil
}

func createKubeConfig(tok *bootstraptokenv1.BootstrapTokenString, role, joinURL, certPath, outDir string) error {
	caCert, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("error reading certificate: %w", err)
	}

	var userName string
	switch role {
	case token.RoleWorker:
		userName = token.WorkerTokenAuthName
	case token.RoleController:
		userName = token.ControllerTokenAuthName
	default:
		return fmt.Errorf("unknown role: %s", role)
	}
	kubeconfig, err := token.GenerateKubeconfig(joinURL, caCert, userName, tok)
	if err != nil {
		return fmt.Errorf("error generating kubeconfig: %w", err)
	}

	encodedToken, err := token.JoinEncode(bytes.NewReader(kubeconfig))
	if err != nil {
		return fmt.Errorf("error encoding token: %w", err)
	}

	err = file.WriteContentAtomically(filepath.Join(outDir, "token_"+tok.ID), []byte(encodedToken), 0640)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig: %w", err)
	}

	return nil
}
