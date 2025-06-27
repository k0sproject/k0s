// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"context"
)

// TODO: decide on renaming root.RootConfig -> root.Config
type RootConfig struct {
	InvocationID        string
	KubeConfig          string
	K0sDataDir          string
	KubeletExtraArgs    string
	Mode                string
	ManagerPort         int
	MetricsBindAddr     string
	HealthProbeBindAddr string
	ExcludeFromPlans    []string
}

// Root is the 'root' of all controllers
type Root interface {
	Run(ctx context.Context) error
}
