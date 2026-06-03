//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	"github.com/k0sproject/k0s/pkg/autopilot/constant"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	applycoordinationv1 "k8s.io/client-go/applyconfigurations/coordination/v1"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RegisterControllers registers all of the autopilot controllers used for updating `k0s`
// to the controller-runtime manager.
func RegisterControllers(ctx context.Context, logger *logrus.Entry, mgr crman.Manager, delegate apdel.ControllerDelegate, enableWorker bool, clusterID string, leaseStatus leaderelection.Status) error {
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

	if enableWorker {
		// Stamp the node lease with the central cordoning label to signal to
		// the autopilot controller that this worker supports central cordoning.
		// Note that if k0s is downgraded to a version that no longer supports
		// central cordoning, the label will remain in place, and there will be
		// no way to detect that this has become stale.
		c := mgr.GetClient()
		if err := c.Apply(ctx, applycoordinationv1.
			Lease(hostname, corev1.NamespaceNodeLease).
			WithLabels(map[string]string{constant.CentralCordoningLabel: ""}),
			client.FieldOwner("k0s/autopilot"), client.ForceOwnership); err != nil {
			return fmt.Errorf("unable to apply lease labels: %w", err)
		}

		// The kubelet only sets the holder identity when creating a new lease
		// from scratch, not when renewing an existing one. The above apply call
		// may very well happen before the kubelet has had a chance to create
		// the lease. As a result, the kubelet finds an existing lease in place
		// and only updates the renewal time, leaving the holder identity unset.
		//
		// Work around this by sending a conditional patch that will set the
		// holder identity if it has not already been set. If the identity has
		// already been set, the patch will fail with an "Invalid" error. We
		// treat this as a success.
		patch, err := json.Marshal([]struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value string `json:"value,omitempty"`
		}{
			{"test", "/spec/holderIdentity", ""},
			{"add", "/spec/holderIdentity", hostname},
		})
		if err != nil {
			return err
		}

		if err := c.Patch(
			ctx,
			&coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
				Name:      hostname,
				Namespace: corev1.NamespaceNodeLease,
			}},
			client.RawPatch(types.JSONPatchType, patch),
		); err != nil && !apierrors.IsInvalid(err) {
			return fmt.Errorf("failed to ensure that a holder identity is set: %w", err)
		}
	}

	if err := registerSignalController(logger, mgr, signalControllerEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s signal")), delegate, clusterID, k0sVersionHandler); err != nil {
		return fmt.Errorf("unable to register signal controller: %w", err)
	}

	if err := registerDownloading(logger, mgr, downloadEventFilter(hostname, apsigpred.DefaultErrorHandler(logger, "k0s downloading")), delegate, k0sBinaryDir); err != nil {
		return fmt.Errorf("unable to register downloading controller: %w", err)
	}

	// Man, I wish there would be a saner way to do this!
	var nodeDelegate apdel.ControllerDelegate
	if _, isController := delegate.CreateObject().(*autopilotv1beta2.ControlNode); isController {
		nodeDelegate = apdel.NodeControllerDelegate()
	}

	cordoningEventFilter := cordoningEventFilter(apsigpred.DefaultErrorHandler(logger, "k0s cordoning"))
	if leaseStatus != leaderelection.StatusLeading {
		cordoningEventFilter = crpred.And(apsigpred.SignalNamePredicate(hostname), cordoningEventFilter)
	}

	if err := registerCordoning(logger, mgr, cordoningEventFilter, delegate, types.NodeName(hostname), leaseStatus); err != nil {
		return fmt.Errorf("unable to register cordoning controller: %w", err)
	}

	if nodeDelegate != nil {
		if err := registerCordoning(logger, mgr, cordoningEventFilter, nodeDelegate, types.NodeName(hostname), leaseStatus); err != nil {
			return fmt.Errorf("unable to register cordoning node controller: %w", err)
		}
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

	unCordoningEventFilter := unCordoningEventFilter(apsigpred.DefaultErrorHandler(logger, "k0s uncordoning"))
	if leaseStatus != leaderelection.StatusLeading {
		unCordoningEventFilter = crpred.And(apsigpred.SignalNamePredicate(hostname), unCordoningEventFilter)
	}

	if err := registerUncordoning(logger, mgr, unCordoningEventFilter, delegate, types.NodeName(hostname), leaseStatus); err != nil {
		return fmt.Errorf("unable to register uncordoning controller: %w", err)
	}

	if nodeDelegate != nil {
		if err := registerUncordoning(logger, mgr, unCordoningEventFilter, nodeDelegate, types.NodeName(hostname), leaseStatus); err != nil {
			return fmt.Errorf("unable to register uncordoning node controller: %w", err)
		}
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
