package worker

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

type KubeProxy struct {
	K0sVars    constant.CfgVars
	CIDRRange  string
	LogLevel   string
	supervisor supervisor.Supervisor
}

// Init
func (k KubeProxy) Init() error {
	return assets.Stage(k.K0sVars.BinDir, "kube-proxy.exe", constant.BinDirMode)
}

func (k KubeProxy) Run() error {
	node, err := getNodeName()
	if err != nil {
		return fmt.Errorf("can't get hostname: %v", err)
	}
	fmt.Println(31)
	sourceVip, err := getSourceVip()
	if err != nil {
		return fmt.Errorf("can't get source vip: %v", err)
	}
	cmd := k.K0sVars.BinDir + "\\kube-proxy.exe"
	args := []string{
		"--hostname-override=" + node,
		"--v=4",
		"--proxy-mode=kernelspace",
		fmt.Sprintf("--cluster-cidr=%s", k.CIDRRange),
		"--network-name=Calico", // TODO: this is the default name
		fmt.Sprintf("--kubeconfig=%s", "c:\\CalicoWindows\\calico-kube-config"),
		fmt.Sprintf("--v=%s", k.LogLevel),
		fmt.Sprintf("--source-vip=%s", strings.TrimSpace(sourceVip)),
		"--feature-gates=WinOverlay=true",
	}
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args,
	}
	k.supervisor.Supervise()
	return nil
}

func (k KubeProxy) Stop() error {
	return k.supervisor.Stop()
}

// Reconcile detects changes in configuration and applies them to the component
func (k KubeProxy) Reconcile() error {
	logrus.Debug("reconcile method called for: worker KubeProxy")
	return nil
}

func (k KubeProxy) Healthy() error {
	return nil
}
