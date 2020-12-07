package worker

import (
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
)

type KubeProxy struct {
	
}

func (k KubeProxy) Init() error {
	err := assets.Stage(constant.BinDir, "kube-proxy.exe", constant.BinDirMode, constant.Group)

	panic(1)
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
