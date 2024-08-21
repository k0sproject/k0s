// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"context"
)

// TODO: decide on renaming root.RootConfig -> root.Config
// nolint:revive
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
