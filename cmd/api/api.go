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

package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	internallog "github.com/k0sproject/k0s/internal/pkg/log"
	mw "github.com/k0sproject/k0s/internal/pkg/middleware"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/etcd"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Run the controller API",
		Long: `Run the controller API.
Reads the runtime configuration from standard input.`,
		Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetOutput(cmd.OutOrStdout())
			internallog.SetInfoLevel()
			return config.CallParentPersistentPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			var run func() error

			if runtimeConfig, err := loadRuntimeConfig(cmd.InOrStdin()); err != nil {
				return err
			} else if run, err = buildServer(runtimeConfig.Spec.K0sVars, runtimeConfig.Spec.NodeConfig); err != nil {
				return err
			}

			return run()
		},
	}

	flags := cmd.Flags()
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		switch f.Name {
		case "debug", "debugListenOn", "verbose":
			flags.AddFlag(f)
		}
	})

	return cmd
}

func loadRuntimeConfig(stdin io.Reader) (*config.RuntimeConfig, error) {
	logrus.Info("Reading runtime configuration from standard input ...")
	bytes, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read from standard input: %w", err)
	}

	runtimeConfig, err := config.ParseRuntimeConfig(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime configuration: %w", err)
	}

	return runtimeConfig, nil
}

func buildServer(k0sVars *config.CfgVars, nodeConfig *v1beta1.ClusterConfig) (func() error, error) {
	// Single kube client for whole lifetime of the API
	client, err := kubeutil.NewClientFromFile(k0sVars.AdminKubeConfigPath)
	if err != nil {
		return nil, err
	}
	secrets := client.CoreV1().Secrets("kube-system")

	prefix := "/v1beta1"
	mux := http.NewServeMux()
	storage := nodeConfig.Spec.Storage

	if storage.Type == v1beta1.EtcdStorageType && !storage.Etcd.IsExternalClusterUsed() {
		// Only mount the etcd handler if we're running on internal etcd storage
		// by default the mux will return 404 back which the caller should handle
		mux.Handle(prefix+"/etcd/members", mw.AllowMethods(http.MethodPost)(
			authMiddleware(etcdHandler(k0sVars.CertRootDir, k0sVars.EtcdCertDir), secrets, "usage-controller-join")))
	}

	if storage.IsJoinable() {
		mux.Handle(prefix+"/ca", mw.AllowMethods(http.MethodGet)(
			authMiddleware(caHandler(k0sVars.CertRootDir), secrets, "usage-controller-join")))
	}

	srv := &http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", nodeConfig.Spec.API.K0sAPIPort),
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			CipherSuites: constant.AllowedTLS12CipherSuiteIDs,
		},
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	cert := filepath.Join(k0sVars.CertRootDir, "k0s-api.crt")
	key := filepath.Join(k0sVars.CertRootDir, "k0s-api.key")

	return func() error { return srv.ListenAndServeTLS(cert, key) }, nil
}

func etcdHandler(certRootDir, etcdCertDir string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		var etcdReq v1beta1.EtcdRequest
		err := json.NewDecoder(req.Body).Decode(&etcdReq)
		if err != nil {
			sendError(err, resp)
			return
		}
		logrus.Infof("etcd API, adding new member: %s", etcdReq.PeerAddress)
		err = etcdReq.Validate()
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdClient, err := etcd.NewClient(certRootDir, etcdCertDir, nil)
		if err != nil {
			sendError(err, resp)
			return
		}

		memberList, err := etcdClient.AddMember(ctx, etcdReq.Node, etcdReq.PeerAddress)
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdResp := v1beta1.EtcdResponse{
			InitialCluster: memberList,
		}

		etcdCaCertPath, etcdCaCertKey := filepath.Join(etcdCertDir, "ca.crt"), filepath.Join(etcdCertDir, "ca.key")
		etcdCACert, err := os.ReadFile(etcdCaCertPath)
		if err != nil {
			sendError(err, resp)
			return
		}
		etcdCAKey, err := os.ReadFile(etcdCaCertKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdResp.CA = v1beta1.CaResponse{
			Key:  etcdCAKey,
			Cert: etcdCACert,
		}
		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(etcdResp); err != nil {
			sendError(err, resp)
			return
		}
	})
}

func caHandler(certRootDir string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		caResp := v1beta1.CaResponse{}
		key, err := os.ReadFile(path.Join(certRootDir, "ca.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Key = key
		crt, err := os.ReadFile(path.Join(certRootDir, "ca.crt"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Cert = crt

		saKey, err := os.ReadFile(path.Join(certRootDir, "sa.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.SAKey = saKey

		saPub, err := os.ReadFile(path.Join(certRootDir, "sa.pub"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.SAPub = saPub

		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(caResp); err != nil {
			sendError(err, resp)
			return
		}
	})
}

// The token is in form of xyz.foobar where:
//   - xyz: the token "ID" in kube api
//   - foobar: the token itself
//
// We need to validate:
//   - that we find a secret with the ID
//   - that the token matches whats inside the secret
func isValidToken(ctx context.Context, secrets clientcorev1.SecretInterface, token string, usage string) bool {
	parts := strings.Split(token, ".")
	logrus.Debugf("token parts: %v", parts)
	if len(parts) != 2 {
		return false
	}

	secretName := "bootstrap-token-" + parts[0]
	secret, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get bootstrap token: %s", err.Error())
		return false
	}

	if string(secret.Data["token-secret"]) != parts[1] {
		return false
	}

	usageValue, ok := secret.Data[usage]
	if !ok || string(usageValue) != "true" {
		return false
	}

	return true
}

func authMiddleware(next http.Handler, secrets clientcorev1.SecretInterface, usage string) http.Handler {
	unauthorizedErr := errors.New("go away")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			sendError(unauthorizedErr, w, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(auth, "Bearer ")
		if len(parts) == 2 {
			token := parts[1]
			if !isValidToken(r.Context(), secrets, token, usage) {
				sendError(unauthorizedErr, w, http.StatusUnauthorized)
				return
			}
		} else {
			sendError(unauthorizedErr, w, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
