package server

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"path"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
)

// APIServer implement the component interface to run kube api
type APIServer struct {
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
	uid        int
	gid        int
}

var apiDefaultArgs = map[string]string{
	"allow-privileged":                   "true",
	"enable-admission-plugins":           "NodeRestriction",
	"requestheader-extra-headers-prefix": "X-Remote-Extra-",
	"requestheader-group-headers":        "X-Remote-Group",
	"requestheader-username-headers":     "X-Remote-User",
	"secure-port":                        "6443",
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
	a.uid, err = util.GetUID(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-apiserver as root"))
	}
	a.gid, _ = util.GetGID(constant.Group)

	return assets.Stage(constant.BinDir, "kube-apiserver", constant.BinDirMode, constant.Group)
}

// Run runs kube api
func (a *APIServer) Run() error {
	err := a.writeKonnectivityConfig()
	if err != nil {
		return err
	}

	logrus.Info("Starting kube-apiserver")
	args := map[string]string{
		"advertise-address":                a.ClusterConfig.Spec.API.Address,
		"authorization-mode":               "Node,RBAC",
		"client-ca-file":                   path.Join(constant.CertRoot, "ca.crt"),
		"enable-bootstrap-token-auth":      "true",
		"kubelet-client-certificate":       path.Join(constant.CertRoot, "apiserver-kubelet-client.crt"),
		"kubelet-client-key":               path.Join(constant.CertRoot, "apiserver-kubelet-client.key"),
		"kubelet-preferred-address-types":  "InternalIP,ExternalIP,Hostname",
		"proxy-client-cert-file":           path.Join(constant.CertRoot, "front-proxy-client.crt"),
		"proxy-client-key-file":            path.Join(constant.CertRoot, "front-proxy-client.key"),
		"requestheader-allowed-names":      "front-proxy-client",
		"requestheader-client-ca-file":     path.Join(constant.CertRoot, "front-proxy-ca.crt"),
		"service-account-key-file":         path.Join(constant.CertRoot, "sa.pub"),
		"service-cluster-ip-range":         a.ClusterConfig.Spec.Network.ServiceCIDR,
		"tls-cert-file":                    path.Join(constant.CertRoot, "server.crt"),
		"tls-private-key-file":             path.Join(constant.CertRoot, "server.key"),
		"egress-selector-config-file":      path.Join(constant.DataDir, "konnectivity.conf"),
		"service-account-signing-key-file": path.Join(constant.CertRoot, "sa.key"),
		"service-account-issuer":           "api",
		"api-audiences":                    "system:konnectivity-server",
		"insecure-port":                    "0",
	}
	for name, value := range a.ClusterConfig.Spec.API.ExtraArgs {
		if args[name] != "" {
			return fmt.Errorf("cannot override apiserver flag: %s", name)
		}
		args[name] = value
	}
	for name, value := range apiDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	apiServerArgs := []string{}
	for name, value := range args {
		apiServerArgs = append(apiServerArgs, fmt.Sprintf("--%s=%s", name, value))
	}

	a.supervisor = supervisor.Supervisor{
		Name:    "kube-apiserver",
		BinPath: assets.BinPath("kube-apiserver"),
		Args:    apiServerArgs,
		UID:     a.uid,
		GID:     a.gid,
	}
	switch a.ClusterConfig.Spec.Storage.Type {
	case config.KineStorageType:
		a.supervisor.Args = append(a.supervisor.Args,
			fmt.Sprintf("--etcd-servers=unix://%s", path.Join(constant.RunDir, "kine.sock:2379"))) // kine endpoint
	case config.EtcdStorageType:
		a.supervisor.Args = append(a.supervisor.Args,
			"--etcd-servers=https://127.0.0.1:2379",
			fmt.Sprintf("--etcd-cafile=%s", path.Join(constant.CertRoot, "etcd/ca.crt")),
			fmt.Sprintf("--etcd-certfile=%s", path.Join(constant.CertRoot, "apiserver-etcd-client.crt")),
			fmt.Sprintf("--etcd-keyfile=%s", path.Join(constant.CertRoot, "apiserver-etcd-client.key")))
	default:
		return errors.New(fmt.Sprintf("invalid storage type: %s", a.ClusterConfig.Spec.Storage.Type))
	}

	a.supervisor.Supervise()

	return nil
}

func (a *APIServer) writeKonnectivityConfig() error {
	tw := util.TemplateWriter{
		Name:     "konnectivity",
		Template: egressSelectorConfigTemplate,
		Data: egressSelectorConfig{
			UDSName: path.Join(constant.RunDir, "konnectivity-server.sock"),
		},
		Path: path.Join(constant.DataDir, "konnectivity.conf"),
	}
	err := tw.Write()
	if err != nil {
		return errors.Wrap(err, "failed to write konnectivity config")
	}

	return nil
}

// Stop stops kine
func (a *APIServer) Stop() error {
	return a.supervisor.Stop()
}
