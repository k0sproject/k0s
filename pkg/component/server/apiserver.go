package server

import (
	"fmt"
	"path"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ApiServer implement the component interface to run kube api
type ApiServer struct {
	ClusterConfig *config.ClusterConfig

	supervisor supervisor.Supervisor
	uid        int
	gid        int
}

// Init extracts needed binaries
func (a *ApiServer) Init() error {
	var err error
	a.uid, err = util.GetUid(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-apiserver as root"))
	}
	a.gid, _ = util.GetGid(constant.Group)

	return assets.Stage(constant.DataDir, path.Join("bin", "kube-apiserver"), constant.Group)
}

// Run runs kube api
func (a *ApiServer) Run() error {
	logrus.Info("Starting kube-apiserver")
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-apiserver",
		BinPath: assets.StagedBinPath(constant.DataDir, "kube-apiserver"),
		Args: []string{
			"--allow-privileged=true",
			"--authorization-mode=Node,RBAC",
			fmt.Sprintf("--client-ca-file=%s", path.Join(constant.CertRoot, "ca.crt")),
			"--enable-admission-plugins=NodeRestriction",
			"--enable-bootstrap-token-auth=true",
			"--insecure-port=0",
			fmt.Sprintf("--kubelet-client-certificate=%s", path.Join(constant.CertRoot, "apiserver-kubelet-client.crt")),
			fmt.Sprintf("--kubelet-client-key=%s", path.Join(constant.CertRoot, "apiserver-kubelet-client.key")),
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			fmt.Sprintf("--proxy-client-cert-file=%s", path.Join(constant.CertRoot, "front-proxy-client.crt")),
			fmt.Sprintf("--proxy-client-key-file=%s", path.Join(constant.CertRoot, "front-proxy-client.key")),
			"--requestheader-allowed-names=front-proxy-client",
			fmt.Sprintf("--requestheader-client-ca-file=%s", path.Join(constant.CertRoot, "front-proxy-ca.crt")),
			"--requestheader-extra-headers-prefix=X-Remote-Extra-",
			"--requestheader-group-headers=X-Remote-Group",
			"--requestheader-username-headers=X-Remote-User",
			"--secure-port=6443",
			fmt.Sprintf("--service-account-key-file=%s", path.Join(constant.CertRoot, "sa.pub")),
			fmt.Sprintf("--service-cluster-ip-range=%s", a.ClusterConfig.Spec.Network.ServiceCIDR),
			fmt.Sprintf("--tls-cert-file=%s", path.Join(constant.CertRoot, "server.crt")),
			fmt.Sprintf("--tls-private-key-file=%s", path.Join(constant.CertRoot, "server.key")),
			"--enable-bootstrap-token-auth",
		},
		Uid: a.uid,
		Gid: a.gid,
	}
	switch a.ClusterConfig.Spec.Storage.Type {
	case "kine":
		a.supervisor.Args = append(a.supervisor.Args, "--etcd-servers=unix:///run/kine.sock:2379") // kine endpoint
	case "etcd":
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

// Stop stops kine
func (a *ApiServer) Stop() error {
	return a.supervisor.Stop()
}
