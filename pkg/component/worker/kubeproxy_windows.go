/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package worker

import (
	"context"
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

type KubeProxy struct {
	K0sVars    *config.CfgVars
	CIDRRange  string
	LogLevel   string
	supervisor supervisor.Supervisor
}

var _ manager.Component = (*KubeProxy)(nil)

// Init
func (k KubeProxy) Init(_ context.Context) error {
	return assets.Stage(k.K0sVars.BinDir, "kube-proxy.exe", constant.BinDirMode)
}

func (k KubeProxy) Start(ctx context.Context) error {
	node, err := getNodeName(ctx)
	if err != nil {
		return fmt.Errorf("can't get hostname: %v", err)
	}

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
