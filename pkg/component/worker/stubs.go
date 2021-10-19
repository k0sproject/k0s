//go:build !windows
// +build !windows

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/constant"
)

type CalicoInstaller struct {
	Token      string
	APIAddress string
	CIDRRange  string
	ClusterDNS string
}

func (c CalicoInstaller) Init() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Run(_ context.Context) error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Stop() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Healthy() error {
	panic("stub component is used: CalicoInstaller")
}

func (c CalicoInstaller) Reconcile() error {
	panic("stub component is used: CalicoInstaller")
}

type KubeProxy struct {
	K0sVars   constant.CfgVars
	CIDRRange string
	LogLevel  string
}

func (k KubeProxy) Init() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Run(_ context.Context) error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Stop() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Reconcile() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Healthy() error {
	panic("stub component is used: KubeProxy")
}
