//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/worker"
)

func initLogging(context.Context, string) error { return nil }

func (p *platformSpecificComponents) addTo(ctx context.Context, m *manager.Manager) {
	if !p.workerConfig.AutopilotDisabled && p.controller == nil {
		m.Add(ctx, &worker.Autopilot{
			K0sVars:       p.k0sVars,
			ClientFactory: p.clientFactory,
		})
	}
}
