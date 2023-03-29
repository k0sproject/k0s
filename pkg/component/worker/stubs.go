//go:build !windows
// +build !windows

/*
Copyright 2022 k0s authors

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

	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
)

type CalicoInstaller struct {
	Token      string
	APIAddress string
	CIDRRange  string
	ClusterDNS string
}

var _ component.Component = (*CalicoInstaller)(nil)

func (c CalicoInstaller) Init(_ context.Context) error {
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

type KubeProxy struct {
	K0sVars   constant.CfgVars
	CIDRRange string
	LogLevel  string
}

var _ component.Component = (*KubeProxy)(nil)

func (k KubeProxy) Init(_ context.Context) error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Run(_ context.Context) error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Stop() error {
	panic("stub component is used: KubeProxy")
}

func (k KubeProxy) Healthy() error {
	panic("stub component is used: KubeProxy")
}
