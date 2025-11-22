//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	"github.com/k0sproject/k0s/pkg/component/status"

	"github.com/sirupsen/logrus"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
)

// RegisterControllers registers all of the autopilot controllers used for updating `k0s`
// to the controller-runtime manager.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, clusterID string) error {
	logger = logger.WithField("controller", delegate.Name())

	hostname, err := apcomm.FindEffectiveHostname()
	if err != nil {
		return fmt.Errorf("unable to determine hostname: %w", err)
	}

	k0sBinaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine k0s binary path: %w", err)
	}
	k0sBinaryDir := filepath.Dir(k0sBinaryPath)

	logger.Infof("Using effective hostname = '%v'", hostname)

	k0sVersionHandler := func() (string, error) {
		return getK0sVersion(status.DefaultSocketPath)
	}

	if err := registerSignalController(logger, mgr, signalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s signal")), delegate, clusterID, k0sVersionHandler); err != nil {
		return fmt.Errorf("unable to register signal controller: %w", err)
	}

	if err := registerDownloading(logger, mgr, downloadEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s downloading")), delegate, k0sBinaryDir); err != nil {
		return fmt.Errorf("unable to register downloading controller: %w", err)
	}

	if err := registerCordoning(logger, mgr, cordoningEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s cordoning")), delegate); err != nil {
		return fmt.Errorf("unable to register cordoning controller: %w", err)
	}

	if err := registerApplyingUpdate(logger, mgr, applyingUpdateEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s applying-update")), delegate, k0sBinaryDir); err != nil {
		return fmt.Errorf("unable to register applying-update controller: %w", err)
	}

	if err := registerRestart(logger, mgr, restartEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s restart")), delegate); err != nil {
		return fmt.Errorf("unable to register restart controller: %w", err)
	}

	if err := registerRestarted(logger, mgr, restartedEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s restarted")), delegate); err != nil {
		return fmt.Errorf("unable to register restarted controller: %w", err)
	}

	if err := registerUncordoning(logger, mgr, unCordoningEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s uncordoning")), delegate); err != nil {
		return fmt.Errorf("unable to register uncordoning controller: %w", err)
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
