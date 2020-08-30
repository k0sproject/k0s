package server

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ControllerManager implement the component interface to run kube scheduler
type ControllerManager struct {
	ClusterConfig *config.ClusterConfig
	supervisor    supervisor.Supervisor
	uid           int
	gid           int
}

// Init extracts the needed binaries
func (a *ControllerManager) Init() error {
	var err error
	a.uid, err = util.GetUid(constant.ControllerManagerUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-controller-manager as root"))
	}
	a.gid, _ = util.GetGid(constant.Group)

	// controller manager should be the only component that needs access to
	// ca.key so let it own it.
	os.Chown(path.Join(constant.CertRoot, "ca.key"), a.uid, -1)

	return assets.Stage(constant.DataDir, path.Join("bin", "kube-controller-manager"), constant.Group)
}

// Run runs kube ControllerManager
func (a *ControllerManager) Run() error {
	logrus.Info("Starting kube-ccm")
	ccmAuthConf := filepath.Join(constant.CertRoot, "ccm.conf")
	args := []string{
		"--allocate-node-cidrs=true",
		fmt.Sprintf("--authentication-kubeconfig=%s", ccmAuthConf),
		fmt.Sprintf("--authorization-kubeconfig=%s", ccmAuthConf),
		fmt.Sprintf("--kubeconfig=%s", ccmAuthConf),
		"--bind-address=127.0.0.1",
		fmt.Sprintf("--client-ca-file=%s", path.Join(constant.CertRoot, "ca.crt")),
		fmt.Sprintf("--cluster-cidr=%s", a.ClusterConfig.Spec.Network.PodCIDR),
		"--cluster-name=mke",
		fmt.Sprintf("--cluster-signing-cert-file=%s", path.Join(constant.CertRoot, "ca.crt")),
		fmt.Sprintf("--cluster-signing-key-file=%s", path.Join(constant.CertRoot, "ca.key")),
		"--controllers=*,bootstrapsigner,tokencleaner",
		"--enable-hostpath-provisioner=true",
		"--leader-elect=true",
		"--node-cidr-mask-size=24",
		fmt.Sprintf("--requestheader-client-ca-file=%s", path.Join(constant.CertRoot, "front-proxy-ca.crt")),
		fmt.Sprintf("--root-ca-file=%s", path.Join(constant.CertRoot, "ca.crt")),
		fmt.Sprintf("--service-account-private-key-file=%s", path.Join(constant.CertRoot, "sa.key")),
		fmt.Sprintf("--service-cluster-ip-range=%s", a.ClusterConfig.Spec.Network.ServiceCIDR),
		"--use-service-account-credentials=true",
		"--controllers=*,tokencleaner",
	}
	for _, arg := range a.ClusterConfig.Spec.ControllerManager.ExtraArgs {
		args = append(args, arg)
	}
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-ccm",
		BinPath: assets.StagedBinPath(constant.DataDir, "kube-controller-manager"),
		Args:    args,
		Uid:     a.uid,
		Gid:     a.gid,
	}

	a.supervisor.Supervise()

	return nil
}

// Stop stops ControllerManager
func (a *ControllerManager) Stop() error {
	return a.supervisor.Stop()
}
