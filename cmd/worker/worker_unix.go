//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
