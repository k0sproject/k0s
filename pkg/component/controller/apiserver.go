// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/featuregates"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// APIServer implement the component interface to run kube api
type APIServer struct {
	NodeConfig                *v1beta1.ClusterConfig
	K0sVars                   *config.CfgVars
	LogLevel                  string
	EnableKonnectivity        bool
	DisableEndpointReconciler bool
	StopTimeout               time.Duration

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

// The egress selector configuration file that connects kube-apiserver to
// konnectivity-server.
type egressSelectorConfig struct {
	Path    string // Where kube-apiserver expects the file
	UDSName string // UDS socket of konnectivity-server
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

// The kube-apiserver launch config.
type apiServerConfig struct {
	flags          stringmap.StringMap   // CLI flags without the leading dashes
	rawArgs        []string              // Raw arguments appended after the flags
	stopTimeout    time.Duration         // How long to wait for kube-apiserver to terminate gracefully
	egressSelector *egressSelectorConfig // The egress selector config, if konnectivity is enabled
}

// Computes the kube-apiserver launch config from the k0s configuration.
// Doesn't write anything to disk.
func (a *APIServer) buildConfig() (*apiServerConfig, error) {
	args := stringmap.StringMap{
		"advertise-address":                a.NodeConfig.Spec.API.Address,
		"secure-port":                      strconv.Itoa(a.NodeConfig.Spec.API.Port),
		"authorization-mode":               "Node,RBAC",
		"client-ca-file":                   filepath.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"enable-bootstrap-token-auth":      "true",
		"kubelet-client-certificate":       filepath.Join(a.K0sVars.CertRootDir, "apiserver-kubelet-client.crt"),
		"kubelet-client-key":               filepath.Join(a.K0sVars.CertRootDir, "apiserver-kubelet-client.key"),
		"kubelet-preferred-address-types":  "InternalIP,ExternalIP,Hostname",
		"proxy-client-cert-file":           filepath.Join(a.K0sVars.CertRootDir, "front-proxy-client.crt"),
		"proxy-client-key-file":            filepath.Join(a.K0sVars.CertRootDir, "front-proxy-client.key"),
		"requestheader-allowed-names":      "front-proxy-client",
		"requestheader-client-ca-file":     filepath.Join(a.K0sVars.CertRootDir, "front-proxy-ca.crt"),
		"service-account-key-file":         filepath.Join(a.K0sVars.CertRootDir, "sa.pub"),
		"service-cluster-ip-range":         a.NodeConfig.Spec.Network.BuildServiceCIDR(a.NodeConfig.Spec.PrimaryAddressFamily()),
		"tls-min-version":                  "VersionTLS12",
		"tls-cert-file":                    filepath.Join(a.K0sVars.CertRootDir, "server.crt"),
		"tls-private-key-file":             filepath.Join(a.K0sVars.CertRootDir, "server.key"),
		"service-account-signing-key-file": filepath.Join(a.K0sVars.CertRootDir, "sa.key"),
		"service-account-issuer":           "https://kubernetes.default.svc",
		"service-account-jwks-uri":         "https://kubernetes.default.svc/openid/v1/jwks",
		"profiling":                        "false",
		"v":                                a.LogLevel,
		"kubelet-certificate-authority":    filepath.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"enable-admission-plugins":         "NodeRestriction",
	}

	if a.NodeConfig.Spec.API.OnlyBindToAddress {
		args["bind-address"] = a.NodeConfig.Spec.API.Address
	}

	apiAudiences := []string{"https://kubernetes.default.svc"}

	var egressSelector *egressSelectorConfig
	if a.EnableKonnectivity {
		egressSelector = &egressSelectorConfig{
			Path:    filepath.Join(a.K0sVars.DataDir, "konnectivity.conf"),
			UDSName: filepath.Join(a.K0sVars.KonnectivitySocketDir, "konnectivity-server.sock"),
		}
		args["egress-selector-config-file"] = egressSelector.Path
		apiAudiences = append(apiAudiences, "system:konnectivity-server")
	}

	args["api-audiences"] = strings.Join(apiAudiences, ",")

	if err := addEtcdArgs(args, a.NodeConfig.Spec.Storage, a.K0sVars); err != nil {
		return nil, err
	}

	for name, value := range a.NodeConfig.Spec.API.ExtraArgs {
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding apiserver flag with user provided value: %s", name)
		}
		args[name] = value
	}
	args = featuregates.ToArgs(args, a.NodeConfig.Spec.FeatureGates, kubeAPIComponentName)

	// kube-apiserver refuses to start if the --anonymous-auth flag is set
	// while the file referenced by --authentication-config contains the
	// anonymous field. Skip the anonymous-auth default in that case, so that
	// anonymous authentication can be managed via the configuration file.
	anonymousAuthManaged := false
	if path := args["authentication-config"]; path != "" {
		var err error
		anonymousAuthManaged, err = authenticationConfigHasAnonymous(path)
		if err != nil {
			logrus.WithError(err).Warn("Failed to check the authentication configuration for the anonymous field, applying the anonymous-auth default")
		} else if anonymousAuthManaged && args["anonymous-auth"] != "" {
			logrus.Warn("The anonymous-auth flag is set while the authentication configuration contains the anonymous field, kube-apiserver will refuse to start")
		}
	}

	for name, value := range apiDefaultArgs {
		if name == "anonymous-auth" && anonymousAuthManaged {
			continue
		}
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

	stopTimeout := a.StopTimeout

	// If the timeout hasn't been specified, do a
	// best guess based on the API server flags.
	if stopTimeout <= 0 {
		requestTimeout := 1 * time.Minute
		if value, ok := args["request-timeout"]; ok {
			if parsed, err := time.ParseDuration(value); err == nil {
				requestTimeout = parsed
			}
		}

		watchTerminationGrace := 0 * time.Second
		if value, ok := args["shutdown-watch-termination-grace-period"]; ok {
			if parsed, err := time.ParseDuration(value); err == nil {
				watchTerminationGrace = parsed
			}
		}

		stopTimeout = max(requestTimeout, watchTerminationGrace) + (2 * time.Second)

		// Clamp the timeout between 5 and 20 seconds. We can't wait for too long
		// currently because the init system will likely kill the process otherwise.
		stopTimeout = max(5*time.Second, min(stopTimeout, 20*time.Second))
	}

	// Enable the API server's watch-drain facility on shutdown, if that flag
	// hasn't been specified by the user. Without this flag, the API server will
	// almost always encounter the request timeout if anything is connected to
	// it via client-go watches. These have a timeout of between five and ten
	// minutes. Note that other types of long-running requests, such as log
	// streams, can still prevent a timely shutdown. However, there's not much
	// that can be done about them apart from setting a short request timeout.
	if _, ok := args["shutdown-watch-termination-grace-period"]; !ok {
		if gracePeriod := stopTimeout - 2*time.Second; gracePeriod > 0 {
			args["shutdown-watch-termination-grace-period"] = gracePeriod.String()
		}
	}

	return &apiServerConfig{
		flags:          args,
		rawArgs:        a.NodeConfig.Spec.API.RawArgs,
		stopTimeout:    stopTimeout,
		egressSelector: egressSelector,
	}, nil
}

// Writes all the files that need to be in place before kube-apiserver starts.
func (c *apiServerConfig) writeFiles() error {
	if c.egressSelector != nil {
		tw := templatewriter.TemplateWriter{
			Name:     "konnectivity",
			Template: egressSelectorConfigTemplate,
			Data:     c.egressSelector,
			Path:     c.egressSelector.Path,
		}
		if err := tw.Write(); err != nil {
			return fmt.Errorf("failed to write konnectivity config: %w", err)
		}
	}

	return nil
}

// Run runs kube api
func (a *APIServer) Start(ctx context.Context) error {
	logrus.Info("Starting kube-apiserver")

	cfg, err := a.buildConfig()
	if err != nil {
		return err
	}

	if err := cfg.writeFiles(); err != nil {
		return err
	}

	a.supervisor = &supervisor.Supervisor{
		Name:        kubeAPIComponentName,
		BinPath:     a.executablePath,
		RunDir:      a.K0sVars.RunDir,
		DataDir:     a.K0sVars.DataDir,
		Args:        append(cfg.flags.ToDashedArgs(), cfg.rawArgs...),
		UID:         a.uid,
		TimeoutStop: cfg.stopTimeout,
	}

	// If the API port is less than 1024, the process needs to bind to a privileged port
	if a.NodeConfig.Spec.API.Port < 1024 {
		a.supervisor.RequiredPrivileges.BindsPrivilegedPorts = true
		logrus.Infof("API port %d is less than 1024, granting privilege to bind to privileged ports", a.NodeConfig.Spec.API.Port)
	}

	return a.supervisor.Supervise(ctx)
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
	certFile := filepath.Join(a.K0sVars.CertRootDir, "admin.crt")
	keyFile := filepath.Join(a.K0sVars.CertRootDir, "admin.key")
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	// Load CA cert
	caCert, err := os.ReadFile(filepath.Join(a.K0sVars.CertRootDir, "ca.crt"))
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
	apiAddress := net.JoinHostPort(a.NodeConfig.Spec.API.Address, strconv.Itoa(a.NodeConfig.Spec.API.Port))
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

// authenticationConfigHasAnonymous reports whether the authentication
// configuration file at path contains the anonymous field. If it does,
// kube-apiserver manages anonymous authentication via the configuration file
// and refuses to start when the --anonymous-auth flag is set as well.
func authenticationConfigHasAnonymous(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var authConfig struct {
		Anonymous json.RawMessage `json:"anonymous"`
	}
	if err := yaml.Unmarshal(data, &authConfig); err != nil {
		return false, fmt.Errorf("failed to parse %q: %w", path, err)
	}

	return len(authConfig.Anonymous) > 0 && string(authConfig.Anonymous) != "null", nil
}

// Adds the flags that connect kube-apiserver to its storage backend.
func addEtcdArgs(args stringmap.StringMap, storage *v1beta1.StorageSpec, k0sVars *config.CfgVars) error {
	switch storage.Type {
	case v1beta1.KineStorageType:
		sockURL := url.URL{
			Scheme: "unix", OmitHost: true,
			Path: filepath.ToSlash(k0sVars.KineSocketPath),
		} // kine endpoint
		args["etcd-servers"] = sockURL.String()
	case v1beta1.EtcdStorageType:
		args["etcd-servers"] = storage.Etcd.GetEndpointsAsString()
		if storage.Etcd.IsTLSEnabled() {
			args["etcd-cafile"] = storage.Etcd.GetCaFilePath(k0sVars.EtcdCertDir)
			args["etcd-certfile"] = storage.Etcd.GetCertFilePath(k0sVars.CertRootDir)
			args["etcd-keyfile"] = storage.Etcd.GetKeyFilePath(k0sVars.CertRootDir)
		}
		if storage.Etcd.IsExternalClusterUsed() {
			args["etcd-prefix"] = storage.Etcd.ExternalCluster.EtcdPrefix
		}
	default:
		return fmt.Errorf("invalid storage type: %s", storage.Type)
	}

	return nil
}
