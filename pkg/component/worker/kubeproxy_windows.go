package worker

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

type KubeProxy struct {
	K0sVars    constant.CfgVars
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

func (k KubeProxy) Healthy() error {
	return nil
}
