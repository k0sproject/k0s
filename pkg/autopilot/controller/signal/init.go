//go:build unix

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package signal

import (
	"context"
	"fmt"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal/airgap"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/signal/k0s"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterControllers registers all of the autopilot controllers used by both controller
// and worker modes.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, k0sDataDir string, enableWorker bool, clusterID string, leaseStatus leaderelection.Status, invocationID string) error {
	if err := k0s.RegisterControllers(ctx, logger, mgr, delegate, enableWorker, clusterID, leaseStatus, invocationID); err != nil {
		return fmt.Errorf("unable to register k0s controllers: %w", err)
	}

	if err := airgap.RegisterControllers(ctx, logger, mgr, delegate, k0sDataDir); err != nil {
		return fmt.Errorf("unable to register airgap controllers: %w", err)
	}

	// On controller+worker nodes the autopilot worker component is not started (only the
	// controller component runs). Airgap update signals are written to v1.Node annotations by the
	// plan provider, so the controller must also watch v1.Node objects for airgap signals when
	// acting as an embedded worker.
	if enableWorker {
		if _, isControlNode := delegate.CreateObject().(*apv1beta2.ControlNode); isControlNode {
			if err := airgap.RegisterControllers(ctx, logger, mgr, apdel.NodeControllerDelegate(), k0sDataDir); err != nil {
				return fmt.Errorf("unable to register airgap controllers for embedded worker node: %w", err)
			}
		}
	}

	return nil
}
