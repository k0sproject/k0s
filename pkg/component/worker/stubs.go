// +build !windows

package worker

import "github.com/k0sproject/k0s/pkg/constant"

type CalicoInstaller struct {
	Token      string
	ApiAddress string
}

func (c CalicoInstaller) Init() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Run() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Stop() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Healthy() error {
	panic("stub component is used: CalicoInstaller")
}

type KubeProxy struct {
	K0sVars  constant.CfgVars
	LogLevel string
}

func (k KubeProxy) Init() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Run() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Stop() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Healthy() error {
	panic("stub component is used: KubeProxy")
}
