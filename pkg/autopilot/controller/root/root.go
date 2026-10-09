// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"context"

	apitypes "k8s.io/apimachinery/pkg/types"
)

type RootConfig struct { //nolint:revive // TODO rename to Config
	InvocationID        string
	KubeConfig          string
	K0sDataDir          string
	KubeletExtraArgs    string
	KubeAPIPort         int
	Mode                string
	ManagerPort         int
	MetricsBindAddr     string
	HealthProbeBindAddr string
	ExcludeFromPlans    []string
	NodeName            apitypes.NodeName
}

// Root is the 'root' of all controllers
type Root interface {
	Run(ctx context.Context) error
}
