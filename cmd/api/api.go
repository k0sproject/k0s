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
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	k0slog "github.com/k0sproject/k0s/internal/pkg/log"
	mw "github.com/k0sproject/k0s/internal/pkg/middleware"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/etcd"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command struct {
	*config.CLIOptions
	client kubernetes.Interface
}

func NewAPICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Run the controller API",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetOutput(cmd.OutOrStdout())
			k0slog.SetInfoLevel()
			return config.CallParentPersistentPreRun(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			return (&command{CLIOptions: opts}).start()
		},
	}
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

func (c *command) start() (err error) {
	// Single kube client for whole lifetime of the API
	c.client, err = kubeutil.NewClientFromFile(c.K0sVars.AdminKubeConfigPath)
	if err != nil {
		return err
	}

	prefix := "/v1beta1"
	mux := http.NewServeMux()
	nodeConfig, err := c.K0sVars.NodeConfig()
	if err != nil {
		return err
	}
	storage := nodeConfig.Spec.Storage

	if storage.Type == v1beta1.EtcdStorageType && !storage.Etcd.IsExternalClusterUsed() {
		// Only mount the etcd handler if we're running on internal etcd storage
		// by default the mux will return 404 back which the caller should handle
		mux.Handle(prefix+"/etcd/members", mw.AllowMethods(http.MethodPost)(
			c.authMiddleware(c.etcdHandler(), "usage-controller-join")))
	}

	if storage.IsJoinable() {
		mux.Handle(prefix+"/ca", mw.AllowMethods(http.MethodGet)(
			c.authMiddleware(c.caHandler(), "usage-controller-join")))
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

	return srv.ListenAndServeTLS(
		filepath.Join(c.K0sVars.CertRootDir, "k0s-api.crt"),
		filepath.Join(c.K0sVars.CertRootDir, "k0s-api.key"),
	)
}

func (c *command) etcdHandler() http.Handler {
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

		etcdClient, err := etcd.NewClient(c.K0sVars.CertRootDir, c.K0sVars.EtcdCertDir, nil)
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

		etcdCaCertPath, etcdCaCertKey := filepath.Join(c.K0sVars.EtcdCertDir, "ca.crt"), filepath.Join(c.K0sVars.EtcdCertDir, "ca.key")
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

func (c *command) caHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		caResp := v1beta1.CaResponse{}
		key, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "ca.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Key = key
		crt, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "ca.crt"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Cert = crt

		saKey, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "sa.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.SAKey = saKey

		saPub, err := os.ReadFile(path.Join(c.K0sVars.CertRootDir, "sa.pub"))
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
func (c *command) isValidToken(ctx context.Context, token string, usage string) bool {
	parts := strings.Split(token, ".")
	logrus.Debugf("token parts: %v", parts)
	if len(parts) != 2 {
		return false
	}

	secretName := fmt.Sprintf("bootstrap-token-%s", parts[0])
	secret, err := c.client.CoreV1().Secrets("kube-system").Get(ctx, secretName, metav1.GetOptions{})
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

func (c *command) authMiddleware(next http.Handler, usage string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			sendError(fmt.Errorf("go away"), w, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(auth, "Bearer ")
		if len(parts) == 2 {
			token := parts[1]
			if !c.isValidToken(r.Context(), token, usage) {
				sendError(fmt.Errorf("go away"), w, http.StatusUnauthorized)
				return
			}
		} else {
			sendError(fmt.Errorf("go away"), w, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
