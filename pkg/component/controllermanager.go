package component

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

// ControllerManager implement the component interface to run kube scheduler
type ControllerManager struct {
	supervisor supervisor.Supervisor
}

// Run runs kube ControllerManager
func (a *ControllerManager) Run() error {
	logrus.Info("Starting kube-ccm")
	ccmAuthConf := filepath.Join(constant.CertRoot, "ccm.conf")
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-ccm",
		BinPath: path.Join(constant.DataDir, "bin", "kube-controller-manager"),
		Args: []string{
			"--allocate-node-cidrs=true",
			fmt.Sprintf("--authentication-kubeconfig=%s", ccmAuthConf),
			fmt.Sprintf("--authorization-kubeconfig=%s", ccmAuthConf),
			fmt.Sprintf("--kubeconfig=%s", ccmAuthConf),
			"--bind-address=127.0.0.1",
			"--client-ca-file=/var/lib/mke/pki/ca.crt",
			"--cluster-cidr=10.244.0.0/16",
			"--cluster-name=mke",
			"--cluster-signing-cert-file=/var/lib/mke/pki/ca.crt",
			"--cluster-signing-key-file=/var/lib/mke/pki/ca.key",
			"--controllers=*,bootstrapsigner,tokencleaner",
			"--enable-hostpath-provisioner=true",
			"--leader-elect=true",
			"--node-cidr-mask-size=24",
			"--requestheader-client-ca-file=/var/lib/mke/pki/front-proxy-ca.crt",
			"--root-ca-file=/var/lib/mke/pki/ca.crt",
			"--service-account-private-key-file=/var/lib/mke/pki/sa.key",
			"--service-cluster-ip-range=10.96.0.0/12",
			"--use-service-account-credentials=true",
		},
	}

	a.supervisor.Supervise()

	return nil
}

// Stop stops ControllerManager
func (a *ControllerManager) Stop() error {
	return a.supervisor.Stop()
}
