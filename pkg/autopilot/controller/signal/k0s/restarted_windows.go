// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"

	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// restartedEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func restartedEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return nil
}

// registerRestarted registers the 'restart' controller to the controller-runtime manager.
//
// This controller is only interested in changes to signal nodes where its signaling
// status is marked as `Restart`
func registerRestarted(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	return nil
}
