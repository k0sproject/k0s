// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package airgap

import (
	"context"
	"fmt"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	Downloading = "Downloading"
)

// RegisterControllers registers all of the autopilot controllers used for updating `airgap`
// to the controller-runtime manager.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, k0sDataDir string) error {
	logger = logger.WithField("controller", delegate.Name())

	hostname, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname: %w", err)
	}

	if err := registerSignalController(logger, mgr, SignalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "signal")), delegate); err != nil {
		return fmt.Errorf("unable to register signal controller: %w", err)
	}

	if err := registerDownloading(logger, mgr, SignalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "airgap download")), delegate, k0sDataDir); err != nil {
		return fmt.Errorf("unable to register downloading controller: %w", err)
	}

	return nil
}

// signalDataUpdateCommandAirgapPredicate creates a predicate that ensures that the
// provided SignalData is an 'airgap' update.
func signalDataUpdateCommandAirgapPredicate() apsigpred.SignalDataPredicate {
	return func(signalData apsigv2.SignalData) bool {
		return signalData.Command.AirgapUpdate != nil
	}
}
