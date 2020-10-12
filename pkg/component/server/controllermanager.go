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

var cmDefaultArgs = map[string]string{
	"allocate-node-cidrs":             "true",
	"bind-address":                    "127.0.0.1",
	"cluster-name":                    "mke",
	"controllers":                     "*,bootstrapsigner,tokencleaner",
	"enable-hostpath-provisioner":     "true",
	"leader-elect":                    "true",
	"node-cidr-mask-size":             "24",
	"use-service-account-credentials": "true",
}

// Init extracts the needed binaries
func (a *ControllerManager) Init() error {
	var err error
	a.uid, err = util.GetUID(constant.ControllerManagerUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-controller-manager as root"))
	}
	a.gid, _ = util.GetGID(constant.Group)

	// controller manager should be the only component that needs access to
	// ca.key so let it own it.
	if err := os.Chown(path.Join(constant.CertRoot, "ca.key"), a.uid, -1); err != nil {
		logrus.Warning(errors.Wrap(err, "Can't change permissions for the ca.key"))
	}

	return assets.Stage(constant.BinDir, "kube-controller-manager", constant.BinDirMode, constant.Group)
}

// Run runs kube ControllerManager
func (a *ControllerManager) Run() error {
	logrus.Info("Starting kube-controller-manager")
	ccmAuthConf := filepath.Join(constant.CertRoot, "ccm.conf")
	args := map[string]string{
		"authentication-kubeconfig":        ccmAuthConf,
		"authorization-kubeconfig":         ccmAuthConf,
		"kubeconfig":                       ccmAuthConf,
		"client-ca-file":                   path.Join(constant.CertRoot, "ca.crt"),
		"cluster-cidr":                     a.ClusterConfig.Spec.Network.PodCIDR,
		"cluster-signing-cert-file":        path.Join(constant.CertRoot, "ca.crt"),
		"cluster-signing-key-file":         path.Join(constant.CertRoot, "ca.key"),
		"requestheader-client-ca-file":     path.Join(constant.CertRoot, "front-proxy-ca.crt"),
		"root-ca-file":                     path.Join(constant.CertRoot, "ca.crt"),
		"service-account-private-key-file": path.Join(constant.CertRoot, "sa.key"),
		"service-cluster-ip-range":         a.ClusterConfig.Spec.Network.ServiceCIDR,
	}
	for name, value := range a.ClusterConfig.Spec.ControllerManager.ExtraArgs {
		if args[name] != "" {
			return fmt.Errorf("cannot override kube-controller-manager flag: %s", name)
		}
		args[name] = value
	}
	for name, value := range cmDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	cmArgs := []string{}
	for name, value := range args {
		cmArgs = append(cmArgs, fmt.Sprintf("--%s=%s", name, value))
	}
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-controller-manager",
		BinPath: assets.BinPath("kube-controller-manager"),
		Args:    cmArgs,
		UID:     a.uid,
		GID:     a.gid,
	}

	a.supervisor.Supervise()

	return nil
}

// Stop stops ControllerManager
func (a *ControllerManager) Stop() error {
	return a.supervisor.Stop()
}
