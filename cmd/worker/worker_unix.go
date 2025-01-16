//go:build unix

/*
Copyright 2025 k0s authors

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
	"os"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
)

func addPlatformSpecificComponents(ctx context.Context, m *manager.Manager, k0sVars *config.CfgVars, controller EmbeddingController, certManager *worker.CertificateManager) {
	// if running inside a controller, status component is already running
	if controller == nil {
		m.Add(ctx, &status.Status{
			Prober: prober.DefaultProber,
			StatusInformation: status.K0sStatus{
				Pid:        os.Getpid(),
				Role:       "worker",
				Args:       os.Args,
				Version:    build.Version,
				Workloads:  true,
				SingleNode: false,
				K0sVars:    k0sVars,
				// worker does not have cluster config. this is only shown in "k0s status -o json".
				// todo: if it's needed, a worker side config client can be set up and used to load the config
				ClusterConfig: nil,
			},
			CertManager: certManager,
			Socket:      k0sVars.StatusSocketPath,
		})
	}

	m.Add(ctx, &worker.Autopilot{
		K0sVars:     k0sVars,
		CertManager: certManager,
	})
}
