// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/k0sproject/k0s/cmd/internal"
	mw "github.com/k0sproject/k0s/internal/pkg/middleware"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	tokenutil "k8s.io/cluster-bootstrap/token/util"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewAPICmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Run the controller API",
		Long: `Run the controller API.
Reads the runtime configuration from standard input.`,
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			log := k0scontext.ValueOrElse(ctx, func() logrus.FieldLogger {
				return logrus.StandardLogger()
			})

			var server *http.Server

			if runtimeConfig, err := loadRuntimeConfig(log, cmd.InOrStdin()); err != nil {
				return err
			} else if server, err = buildServer(log, runtimeConfig.Spec.K0sVars, runtimeConfig.Spec.NodeConfig); err != nil {
				return err
			}

			listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
			if err != nil {
				return err
			}
			defer server.Close()

			log.Info("Listening on ", server.Addr, ", start serving")

			doneServing := make(chan struct{})
			go func() {
				defer close(doneServing)
				err = server.ServeTLS(listener, "", "")
			}()

			select {
			case <-doneServing:
				return fmt.Errorf("unexpected server error: %w", err)

			case <-ctx.Done():
				log.Info("Shutting down server: ", context.Cause(ctx))

				ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
				defer cancel()
				if err := server.Shutdown(ctx); err != nil {
					return fmt.Errorf("while shutting down server: %w", err)
				}

				<-doneServing
				if !errors.Is(err, http.ErrServerClosed) {
					return fmt.Errorf("unexpected error after server shutdown: %w", err)
				}

				log.Info("Good bye")
				return nil
			}
		},
	}

	debugFlags.LongRunning().AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		switch f.Name {
		case "debug", "debugListenOn", "verbose":
			flags.AddFlag(f)
		}
	})

	return cmd
}

func loadRuntimeConfig(log logrus.FieldLogger, stdin io.Reader) (*config.RuntimeConfig, error) {
	log.Info("Reading runtime configuration from standard input")
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

func buildServer(log logrus.FieldLogger, k0sVars *config.CfgVars, nodeConfig *v1beta1.ClusterConfig) (*http.Server, error) {
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
			authMiddleware(etcdHandler(log, k0sVars.CertRootDir, k0sVars.EtcdCertDir), log, secrets, "controller-join")))
	}

	if storage.IsJoinable() {
		mux.Handle(prefix+"/ca", mw.AllowMethods(http.MethodGet)(
			authMiddleware(caHandler(k0sVars.CertRootDir), log, secrets, "controller-join")))
	}

	ipAddr, bindAddressSpecified := nodeConfig.Spec.API.ExtraArgs["bind-address"]
	if !bindAddressSpecified && nodeConfig.Spec.API.OnlyBindToAddress {
		ipAddr = nodeConfig.Spec.API.Address
	}

	cert, err := tls.LoadX509KeyPair(
		filepath.Join(k0sVars.CertRootDir, "k0s-api.crt"),
		filepath.Join(k0sVars.CertRootDir, "k0s-api.key"),
	)
	if err != nil {
		return nil, err
	}

	return &http.Server{
		Handler: mux,
		Addr:    net.JoinHostPort(ipAddr, strconv.Itoa(nodeConfig.Spec.API.K0sAPIPort)),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
			CipherSuites: constant.AllowedTLS12CipherSuiteIDs,
		},
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}, nil
}

func etcdHandler(log logrus.FieldLogger, certRootDir, etcdCertDir string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		var etcdReq v1beta1.EtcdRequest
		err := json.NewDecoder(req.Body).Decode(&etcdReq)
		if err != nil {
			sendError(err, resp)
			return
		}
		log.Infof("etcd API, adding new member: %s", etcdReq.PeerAddress)
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
		defer etcdClient.Close()

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
func isValidToken(ctx context.Context, log logrus.FieldLogger, secrets clientcorev1.SecretInterface, rawTokenString, usage string) bool {
	tokenString, err := bootstraptokenv1.NewBootstrapTokenString(rawTokenString)
	if err != nil {
		return false
	}

	secretName := tokenutil.BootstrapTokenSecretName(tokenString.ID)
	secret, err := secrets.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(err).Error("Failed to get bootstrap token with ID ", tokenString.ID)
		}
		return false
	}

	token, err := bootstraptokenv1.BootstrapTokenFromSecret(secret)
	if err != nil {
		log.WithError(err).Errorf("Bootstrap token with ID %s is malformed", tokenString.ID)
		return false
	}

	if token.Expires != nil && !time.Now().Before(token.Expires.Time) {
		return false
	}

	if *token.Token != *tokenString {
		return false
	}

	switch {
	case slices.Contains(token.Usages, usage):
		return true // usage found
	case bytes.Equal(secret.Data["usage-"+usage], []byte("true")):
		return true // usage found in its legacy form
	default:
		return false // usage not found
	}
}

func authMiddleware(next http.Handler, log logrus.FieldLogger, secrets clientcorev1.SecretInterface, usage string) http.Handler {
	unauthorizedErr := errors.New("go away")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if ok && isValidToken(r.Context(), log, secrets, token, usage) {
			next.ServeHTTP(w, r)
		} else {
			sendError(unauthorizedErr, w, http.StatusUnauthorized)
		}
	})
}
