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

package k0s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	"github.com/k0sproject/k0s/pkg/component/status"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	Downloading     = "Downloading"
	Cordoning       = "Cordoning"
	CordoningFailed = "CordoningFailed"
	UnCordoning     = "UnCordoning"
	ApplyingUpdate  = "ApplyingUpdate"
	Restart         = "Restart"
)

// RegisterControllers registers all of the autopilot controllers used for updating `k0s`
// to the controller-runtime manager.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, clusterID string) error {
	logger = logger.WithField("controller", delegate.Name())

	hostname, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname for controlnode 'signal' reconciler: %w", err)
	}

	k0sBinaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine k0s binary path for controlnode 'signal' reconciler: %w", err)
	}
	k0sBinaryDir := filepath.Dir(k0sBinaryPath)

	logger.Infof("Using effective hostname = '%v'", hostname)

	if err := registerSignalController(logger, mgr, signalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s signal")), delegate, clusterID); err != nil {
		return fmt.Errorf("unable to register k0s 'signal' controller: %w", err)
	}

	if err := registerDownloading(logger, mgr, downloadEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s downloading")), delegate, k0sBinaryDir); err != nil {
		return fmt.Errorf("unable to register k0s 'downloading' controller: %w", err)
	}

	if err := registerCordoning(logger, mgr, cordoningEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s cordoning")), delegate); err != nil {
		return fmt.Errorf("unable to register k0s 'cordoning' controller: %w", err)
	}

	if err := registerApplyingUpdate(logger, mgr, applyingUpdateEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s applying-update")), delegate, k0sBinaryDir); err != nil {
		return fmt.Errorf("unable to register k0s 'applying-update' controller: %w", err)
	}

	if err := registerRestart(logger, mgr, restartEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s restart")), delegate); err != nil {
		return fmt.Errorf("unable to register k0s 'restart' controller: %w", err)
	}

	if err := registerRestarted(logger, mgr, restartedEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s restarted")), delegate); err != nil {
		return fmt.Errorf("unable to register k0s 'restarted' controller: %w", err)
	}

	if err := registerUncordoning(logger, mgr, unCordoningEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s uncordoning")), delegate); err != nil {
		return fmt.Errorf("unable to register k0s 'uncordoning' controller: %w", err)
	}

	return nil
}

// getK0sVersion returns the version k0s installed, as identified by the
// provided status socket path.
func getK0sVersion(statusSocketPath string) (string, error) {
	status, err := status.GetStatusInfo(statusSocketPath)
	if err != nil {
		return "", err
	}

	return status.Version, nil
}

// getK0sPid returns the PID of a running k0s based on its status socket.
func getK0sPid(statusSocketPath string) (int, error) {
	status, err := status.GetStatusInfo(statusSocketPath)
	if err != nil {
		return -1, err
	}

	return status.Pid, nil
}

// signalDataUpdateCommandK0sPredicate creates a predicate that ensures that the
// provided SignalData is an 'k0s' update.
func signalDataUpdateCommandK0sPredicate() apsigpred.SignalDataPredicate {
	return func(signalData apsigv2.SignalData) bool {
		return signalData.Command.K0sUpdate != nil
	}
}

func needsCordoning(signalNode client.Object) bool {
	kind := signalNode.GetObjectKind().GroupVersionKind().Kind
	if kind == "Node" {
		return true
	}
	for k, v := range signalNode.GetAnnotations() {
		if k == apconst.K0SControlNodeModeAnnotation && v == apconst.K0SControlNodeModeControllerWorker {
			return true
		}
	}
	return false
}
