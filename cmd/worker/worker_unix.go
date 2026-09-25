//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"

	apitypes "k8s.io/apimachinery/pkg/types"
)

func initLogging(context.Context, string) error { return nil }

func addPlatformSpecificComponents(ctx context.Context, m *manager.Manager, k0sVars *config.CfgVars, nodeName apitypes.NodeName, controller EmbeddingController, certManager *worker.CertificateManager) {
	m.Add(ctx, &worker.Autopilot{
		K0sVars:     k0sVars,
		NodeName:    nodeName,
		CertManager: certManager,
	})
}
