//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"
)

func initLogging(context.Context, string) error { return nil }

func addPlatformSpecificComponents(ctx context.Context, m *manager.Manager, k0sVars *config.CfgVars, workerConfig *workerconfig.Profile, controller EmbeddingController, certManager *worker.CertificateManager) {
	if !workerConfig.AutopilotDisabled && controller == nil {
		m.Add(ctx, &worker.Autopilot{
			K0sVars:     k0sVars,
			CertManager: certManager,
		})
	}
}
