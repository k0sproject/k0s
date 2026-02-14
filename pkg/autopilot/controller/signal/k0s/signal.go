//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"fmt"
	"strings"
	"time"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	SignalResponseProcessingTimeout = 1 * time.Minute
)

type k0sVersionHandlerFunc func() (string, error)

// signalDataUpdateCommandK0sPredicate creates a predicate that ensures that the
// provided SignalData is an 'k0s' update.
func signalDataUpdateCommandK0sPredicate() apsigpred.SignalDataPredicate {
	return func(signalData apsigv2.SignalData) bool {
		return signalData.Command.K0sUpdate != nil
	}
}

// signalControllerEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func signalControllerEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataNoStatusPredicate(),
		),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(ue crev.UpdateEvent) bool {
				return true
			},
		},
	)
}

type signalControllerHandler struct {
	timeout           time.Duration
	clusterID         string
	k0sVersionHandler k0sVersionHandlerFunc
}

// registerSignalController registers the k0s 'signal' controller to the controller-runtime manager.
//
// This controller is only interested in changes to its own annotations, and is the main
// mechanism in identifying incoming autopilot k0s signaling updates.
func registerSignalController(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate, clusterID string, k0sVersionHandler k0sVersionHandlerFunc) error {
	logr := logger.WithFields(logrus.Fields{"updatetype": "k0s"})
	name := strings.ToLower(delegate.Name()) + "_k0s_signal"

	logr.Info("Registering reconciler: ", name)

	return cr.NewControllerManagedBy(mgr).
		Named(name).
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			apsigcomm.NewSignalController(
				logr,
				mgr.GetClient(),
				delegate,
				&signalControllerHandler{
					timeout:           SignalResponseProcessingTimeout,
					clusterID:         clusterID,
					k0sVersionHandler: k0sVersionHandler,
				},
			),
		)
}

// Handle ensures that the provided `SignalData` is valid, and proceeds to perform k0s version comparison
// to determine if a k0s update is required. If required, the status moves to `Downloading`.
func (h *signalControllerHandler) Handle(ctx context.Context, sctx apsigcomm.SignalControllerContext) (cr.Result, error) {
	// Ensure that the request is not expired.
	if checkExpiredInvalid(sctx.Log, sctx.SignalData, SignalResponseProcessingTimeout) {
		return cr.Result{}, nil
	}

	if sctx.SignalData != nil && sctx.SignalData.Status != nil {
		sctx.Log.Debug("Ignoring signal with status ", sctx.SignalData.Status.Status)
		return cr.Result{}, nil
	}

	sctx.Log.Infof("Found available signaling update request")

	// Confirm that the installed version of k0s requires an update
	// TODO:s0j - make the status path an argument to signaling/plan
	k0sVersion, err := h.k0sVersionHandler()
	if err != nil {
		return cr.Result{}, fmt.Errorf("unable to determine k0s version: %w", err)
	}

	sctx.Log.Infof("Current version of k0s = '%s', requested version = '%s'", k0sVersion, sctx.SignalData.Command.K0sUpdate.Version)

	// Move to 'Completed' if we match versions on a non-forced update. Otherwise, proceed to 'Downloading'
	var status = apsigcomm.Downloading
	if k0sVersion == sctx.SignalData.Command.K0sUpdate.Version && !sctx.SignalData.Command.K0sUpdate.ForceUpdate {
		status = apsigcomm.Completed
	}

	// Populate the response into the annotations
	signalNodeCopy := sctx.Delegate.DeepCopy(sctx.SignalNode)

	var oldStatus string
	if sctx.SignalData.Status != nil {
		oldStatus = sctx.SignalData.Status.Status
	}
	sctx.SignalData.Status = apsigv2.NewStatus(status)
	if err := sctx.SignalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to marshal k0s signal data for node='%s': %w", signalNodeCopy.GetName(), err)
	}

	sctx.Log.Infof("Updating signaling response to '%s'", status)
	if err := sctx.Client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		return cr.Result{}, fmt.Errorf("unable to update k0s signal node='%s' with status='%s': %w", signalNodeCopy.GetName(), status, err)
	}

	_ = apcomm.ReportEvent(&apcomm.Event{
		ClusterID: h.clusterID,
		OldStatus: oldStatus,
		NewStatus: status,
	})

	return cr.Result{}, nil
}

// checkExpiredInvalid ensures that the provided SignalData is not expired/invalid if in 'ApplyingUpdate'
func checkExpiredInvalid(logger *logrus.Entry, signalData *apsigv2.SignalData, timeout time.Duration) bool {
	// If the response is in `ApplyingUpdate`, ensure that it hasn't stayed in this state
	// past our threshold.
	if signalData != nil && signalData.Status != nil && signalData.Status.Status == ApplyingUpdate {
		responseTimestamp, err := time.Parse(time.RFC3339, signalData.Status.Timestamp)
		if err != nil {
			// Failing to parse the timestamp means this response should be considered
			// invalid - can't retry as this timestamp will never be updated.
			logger.Infof("Invalid signaling response timestamp = %v: %v", signalData.Status.Timestamp, err)
			return true
		}

		// Expired
		if time.Now().After(responseTimestamp.Add(timeout)) {
			return true
		}
	}

	return false
}
