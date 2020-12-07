package worker

import (
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
)

type KubeProxy struct {
	K0sVars constant.CfgVars
}

func (k KubeProxy) Init() error {
	err := assets.Stage(k.K0sVars.BinDir, "kube-proxy.exe", constant.BinDirMode)

	if err != nil {
		panic(err)
		return err
	}
	return nil
}

func (k KubeProxy) Run() error {
	panic("implement me")
}

func (k KubeProxy) Stop() error {
	return nil
}

func (k KubeProxy) Healthy() error {
	return nil
}
