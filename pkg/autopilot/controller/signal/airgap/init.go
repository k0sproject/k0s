// Copyright 2022 k0s authors
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
		return fmt.Errorf("unable to determine hostname for controlnode airgap 'signal' reconciler: %w", err)
	}

	if err := registerSignalController(logger, mgr, SignalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "signal")), delegate); err != nil {
		return fmt.Errorf("unable to register 'airgap-signal' controller: %w", err)
	}

	if err := registerDownloading(logger, mgr, SignalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "airgap download")), delegate, k0sDataDir); err != nil {
		return fmt.Errorf("unable to register 'airgap-downloading' controller: %w", err)
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
