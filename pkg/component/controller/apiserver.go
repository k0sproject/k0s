// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// APIServer implement the component interface to run kube api
type APIServer struct {
	ClusterConfig             *v1beta1.ClusterConfig
	K0sVars                   *config.CfgVars
	LogLevel                  string
	Storage                   manager.Component
	EnableKonnectivity        bool
	DisableEndpointReconciler bool

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int
}

var _ manager.Component = (*APIServer)(nil)
var _ manager.Ready = (*APIServer)(nil)

const kubeAPIComponentName = "kube-apiserver"

var apiDefaultArgs = map[string]string{
	"allow-privileged":                   "true",
	"requestheader-extra-headers-prefix": "X-Remote-Extra-",
	"requestheader-group-headers":        "X-Remote-Group",
	"requestheader-username-headers":     "X-Remote-User",
	"secure-port":                        "6443",
	"anonymous-auth":                     "false",
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
func (a *APIServer) Init(_ context.Context) error {
	var err error
	a.uid, err = users.LookupUID(constant.ApiserverUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.ApiserverUser, err)
		a.uid = users.RootUID
		logrus.WithError(err).Warn("Running Kubernetes API server as root")
	}
	a.executablePath, err = assets.StageExecutable(a.K0sVars.BinDir, kubeAPIComponentName)
	return err
}

// Run runs kube api
func (a *APIServer) Start(_ context.Context) error {
	logrus.Info("Starting kube-apiserver")
	args := stringmap.StringMap{
		"advertise-address":                a.ClusterConfig.Spec.API.Address,
		"secure-port":                      strconv.Itoa(a.ClusterConfig.Spec.API.Port),
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
		"service-cluster-ip-range":         a.ClusterConfig.Spec.Network.BuildServiceCIDR(a.ClusterConfig.PrimaryAddressFamily()),
		"tls-min-version":                  "VersionTLS12",
		"tls-cert-file":                    path.Join(a.K0sVars.CertRootDir, "server.crt"),
		"tls-private-key-file":             path.Join(a.K0sVars.CertRootDir, "server.key"),
		"service-account-signing-key-file": path.Join(a.K0sVars.CertRootDir, "sa.key"),
		"service-account-issuer":           "https://kubernetes.default.svc",
		"service-account-jwks-uri":         "https://kubernetes.default.svc/openid/v1/jwks",
		"profiling":                        "false",
		"v":                                a.LogLevel,
		"kubelet-certificate-authority":    path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"enable-admission-plugins":         "NodeRestriction",
	}

	if a.ClusterConfig.Spec.API.OnlyBindToAddress {
		args["bind-address"] = a.ClusterConfig.Spec.API.Address
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
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding apiserver flag with user provided value: %s", name)
		}
		args[name] = value
	}
	args = a.ClusterConfig.Spec.FeatureGates.BuildArgs(args, kubeAPIComponentName)
	for name, value := range apiDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	if args["tls-cipher-suites"] == "" {
		args["tls-cipher-suites"] = constant.AllowedTLS12CipherSuiteNames()
	}

	if a.DisableEndpointReconciler {
		args["endpoint-reconciler-type"] = "none"
	}

	var apiServerArgs []string
	for name, value := range args {
		apiServerArgs = append(apiServerArgs, fmt.Sprintf("--%s=%s", name, value))
	}
	apiServerArgs = append(apiServerArgs, a.ClusterConfig.Spec.API.RawArgs...)

	a.supervisor = &supervisor.Supervisor{
		Name:    kubeAPIComponentName,
		BinPath: a.executablePath,
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    apiServerArgs,
		UID:     a.uid,
	}

	etcdArgs, err := getEtcdArgs(a.ClusterConfig.Spec.Storage, a.K0sVars)
	if err != nil {
		return err
	}
	a.supervisor.Args = append(a.supervisor.Args, etcdArgs...)

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
	if a.supervisor != nil {
		return a.supervisor.Stop()
	}
	return nil
}

// Health-check interface
func (a *APIServer) Ready() error {
	// Load client cert so the api can authenticate the request.
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
	apiAddress := net.JoinHostPort(a.ClusterConfig.Spec.API.Address, strconv.Itoa(a.ClusterConfig.Spec.API.Port))
	resp, err := client.Get(fmt.Sprintf("https://%s/readyz?verbose", apiAddress))
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

func getEtcdArgs(storage *v1beta1.StorageSpec, k0sVars *config.CfgVars) ([]string, error) {
	var args []string

	switch storage.Type {
	case v1beta1.KineStorageType:
		sockURL := url.URL{
			Scheme: "unix", OmitHost: true,
			Path: filepath.ToSlash(k0sVars.KineSocketPath),
		} // kine endpoint
		args = append(args, "--etcd-servers="+sockURL.String())
	case v1beta1.EtcdStorageType:
		args = append(args, "--etcd-servers="+storage.Etcd.GetEndpointsAsString())
		if storage.Etcd.IsTLSEnabled() {
			args = append(args,
				"--etcd-cafile="+storage.Etcd.GetCaFilePath(k0sVars.EtcdCertDir),
				"--etcd-certfile="+storage.Etcd.GetCertFilePath(k0sVars.CertRootDir),
				"--etcd-keyfile="+storage.Etcd.GetKeyFilePath(k0sVars.CertRootDir))
		}
		if storage.Etcd.IsExternalClusterUsed() {
			args = append(args, "--etcd-prefix="+storage.Etcd.ExternalCluster.EtcdPrefix)
		}
	default:
		return nil, fmt.Errorf("invalid storage type: %s", storage.Type)
	}

	return args, nil
}
