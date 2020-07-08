package server

import (
	"fmt"
	"path"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

// ApiServer implement the component interface to run kube api
type ApiServer struct {
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
}

// Init extracts needed binaries
func (a *ApiServer) Init() error {
	return assets.Stage(constant.DataDir, path.Join("bin", "kube-apiserver"))
}

// Run runs kube api
func (a *ApiServer) Run() error {
	logrus.Info("Starting kube-apiserver")
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-apiserver",
		BinPath: path.Join(constant.DataDir, "bin", "kube-apiserver"),
		Args: []string{
			"--allow-privileged=true",
			"--authorization-mode=Node,RBAC",
			"--client-ca-file=/var/lib/mke/pki/ca.crt",
			"--enable-admission-plugins=NodeRestriction",
			"--enable-bootstrap-token-auth=true",
			// "--etcd-cafile=/var/lib/mke/pki/etcd/ca.crt",
			// "--etcd-certfile=/var/lib/mke/pki/apiserver-etcd-client.crt",
			// "--etcd-keyfile=/var/lib/mke/pki/apiserver-etcd-client.key",
			"--etcd-servers=http://127.0.0.1:2379", // kine endpoint
			"--insecure-port=0",
			"--kubelet-client-certificate=/var/lib/mke/pki/apiserver-kubelet-client.crt",
			"--kubelet-client-key=/var/lib/mke/pki/apiserver-kubelet-client.key",
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			"--proxy-client-cert-file=/var/lib/mke/pki/front-proxy-client.crt",
			"--proxy-client-key-file=/var/lib/mke/pki/front-proxy-client.key",
			"--requestheader-allowed-names=front-proxy-client",
			"--requestheader-client-ca-file=/var/lib/mke/pki/front-proxy-ca.crt",
			"--requestheader-extra-headers-prefix=X-Remote-Extra-",
			"--requestheader-group-headers=X-Remote-Group",
			"--requestheader-username-headers=X-Remote-User",
			"--secure-port=6443",
			"--service-account-key-file=/var/lib/mke/pki/sa.pub",
			fmt.Sprintf("--service-cluster-ip-range=%s", a.ClusterConfig.Spec.Network.ServiceCIDR),
			"--tls-cert-file=/var/lib/mke/pki/server.crt",
			"--tls-private-key-file=/var/lib/mke/pki/server.key",
			"--enable-bootstrap-token-auth",
		},
	}

	a.supervisor.Supervise()

	return nil
}

// Stop stops kine
func (a *ApiServer) Stop() error {
	return a.supervisor.Stop()
}
