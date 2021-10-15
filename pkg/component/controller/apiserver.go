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
package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// APIServer implement the component interface to run kube api
type APIServer struct {
	ClusterConfig      *v1beta1.ClusterConfig
	K0sVars            constant.CfgVars
	LogLevel           string
	Storage            component.Component
	EnableKonnectivity bool
	gid                int
	supervisor         supervisor.Supervisor
	uid                int
}

var apiDefaultArgs = map[string]string{
	"allow-privileged":                   "true",
	"requestheader-extra-headers-prefix": "X-Remote-Extra-",
	"requestheader-group-headers":        "X-Remote-Group",
	"requestheader-username-headers":     "X-Remote-User",
	"secure-port":                        "6443",
	"anonymous-auth":                     "false",
	"tls-cipher-suites":                  "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
}

const egressSelectorConfigTemplate = `
apiVersion: apiserver.k8s.io/v1beta1
kind: EgressSelectorConfiguration
egressSelections:
- name: cluster
  connection:
    proxyProtocol: GRPC
    transport:
      uds:
        udsName: {{ .UDSName }}
`

type egressSelectorConfig struct {
	UDSName string
}

// Init extracts needed binaries
func (a *APIServer) Init() error {
	var err error
	a.uid, err = users.GetUID(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running kube-apiserver as root: %w", err))
	}
	return assets.Stage(a.K0sVars.BinDir, "kube-apiserver", constant.BinDirMode)
}

// Run runs kube api
func (a *APIServer) Run(_ context.Context) error {
	logrus.Info("Starting kube-apiserver")
	args := map[string]string{
		"advertise-address":                a.ClusterConfig.Spec.API.Address,
		"secure-port":                      fmt.Sprintf("%d", a.ClusterConfig.Spec.API.Port),
		"authorization-mode":               "Node,RBAC",
		"client-ca-file":                   path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"enable-bootstrap-token-auth":      "true",
		"kubelet-client-certificate":       path.Join(a.K0sVars.CertRootDir, "apiserver-kubelet-client.crt"),
		"kubelet-client-key":               path.Join(a.K0sVars.CertRootDir, "apiserver-kubelet-client.key"),
		"kubelet-preferred-address-types":  "InternalIP,ExternalIP,Hostname",
		"proxy-client-cert-file":           path.Join(a.K0sVars.CertRootDir, "front-proxy-client.crt"),
		"proxy-client-key-file":            path.Join(a.K0sVars.CertRootDir, "front-proxy-client.key"),
		"requestheader-allowed-names":      "front-proxy-client",
		"requestheader-client-ca-file":     path.Join(a.K0sVars.CertRootDir, "front-proxy-ca.crt"),
		"service-account-key-file":         path.Join(a.K0sVars.CertRootDir, "sa.pub"),
		"service-cluster-ip-range":         a.ClusterConfig.Spec.Network.BuildServiceCIDR(a.ClusterConfig.Spec.API.Address),
		"tls-cert-file":                    path.Join(a.K0sVars.CertRootDir, "server.crt"),
		"tls-private-key-file":             path.Join(a.K0sVars.CertRootDir, "server.key"),
		"service-account-signing-key-file": path.Join(a.K0sVars.CertRootDir, "sa.key"),
		"service-account-issuer":           "https://kubernetes.default.svc",
		"service-account-jwks-uri":         "https://kubernetes.default.svc/openid/v1/jwks",
		"insecure-port":                    "0",
		"profiling":                        "false",
		"v":                                a.LogLevel,
		"kubelet-certificate-authority":    path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"enable-admission-plugins":         "NodeRestriction,PodSecurityPolicy",
	}

	apiAudiences := []string{"https://kubernetes.default.svc"}

	if a.EnableKonnectivity {
		err := a.writeKonnectivityConfig()
		if err != nil {
			return err
		}
		args["egress-selector-config-file"] = path.Join(a.K0sVars.DataDir, "konnectivity.conf")
		apiAudiences = append(apiAudiences, "system:konnectivity-server")
	}

	args["api-audiences"] = strings.Join(apiAudiences, ",")

	for name, value := range a.ClusterConfig.Spec.API.ExtraArgs {
		if args[name] != "" {
			logrus.Warnf("overriding apiserver flag with user provided value: %s", name)
		}
		args[name] = value
	}
	a.ClusterConfig.Spec.Network.DualStack.EnableDualStackFeatureGate(args)

	for name, value := range apiDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	if a.ClusterConfig.Spec.API.ExternalAddress != "" || a.ClusterConfig.Spec.API.TunneledNetworkingMode {
		args["endpoint-reconciler-type"] = "none"
	}
	var apiServerArgs []string
	for name, value := range args {
		apiServerArgs = append(apiServerArgs, fmt.Sprintf("--%s=%s", name, value))
	}

	a.supervisor = supervisor.Supervisor{
		Name:    "kube-apiserver",
		BinPath: assets.BinPath("kube-apiserver", a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    apiServerArgs,
		UID:     a.uid,
		GID:     a.gid,
	}
	switch a.ClusterConfig.Spec.Storage.Type {
	case v1beta1.KineStorageType:
		a.supervisor.Args = append(a.supervisor.Args,
			fmt.Sprintf("--etcd-servers=unix://%s", a.K0sVars.KineSocketPath)) // kine endpoint
	case v1beta1.EtcdStorageType:
		a.supervisor.Args = append(a.supervisor.Args,
			"--etcd-servers=https://127.0.0.1:2379",
			fmt.Sprintf("--etcd-cafile=%s", path.Join(a.K0sVars.CertRootDir, "etcd/ca.crt")),
			fmt.Sprintf("--etcd-certfile=%s", path.Join(a.K0sVars.CertRootDir, "apiserver-etcd-client.crt")),
			fmt.Sprintf("--etcd-keyfile=%s", path.Join(a.K0sVars.CertRootDir, "apiserver-etcd-client.key")))
	default:
		return fmt.Errorf("invalid storage type: %s", a.ClusterConfig.Spec.Storage.Type)
	}
	return a.supervisor.Supervise()
}

func (a *APIServer) writeKonnectivityConfig() error {
	tw := templatewriter.TemplateWriter{
		Name:     "konnectivity",
		Template: egressSelectorConfigTemplate,
		Data: egressSelectorConfig{
			UDSName: path.Join(a.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		},
		Path: path.Join(a.K0sVars.DataDir, "konnectivity.conf"),
	}
	err := tw.Write()
	if err != nil {
		return fmt.Errorf("failed to write konnectivity config: %w", err)
	}

	return nil
}

// Stop stops APIServer
func (a *APIServer) Stop() error {
	return a.supervisor.Stop()
}

// Reconcile detects changes in configuration and applies them to the component
func (a *APIServer) Reconcile() error {
	logrus.Debug("reconcile method called for: APIServer")
	return nil
}

// Health-check interface
func (a *APIServer) Healthy() error {
	// Load client cert so the api can authenitcate the request.
	certFile := path.Join(a.K0sVars.CertRootDir, "admin.crt")
	keyFile := path.Join(a.K0sVars.CertRootDir, "admin.key")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	// Load CA cert
	caCert, err := os.ReadFile(path.Join(a.K0sVars.CertRootDir, "ca.crt"))
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/readyz?verbose", a.ClusterConfig.Spec.API.Port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			logrus.Debugf("api server readyz output:\n %s", string(body))
		}
		return fmt.Errorf("expected 200 for api server ready check, got %d", resp.StatusCode)
	}
	return nil
}
