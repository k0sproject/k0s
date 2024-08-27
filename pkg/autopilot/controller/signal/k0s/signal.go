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
	DefaultK0sStatusSocketPath      = "/run/k0s/status.sock"
)

type k0sVersionHandlerFunc func() (string, error)

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
func registerSignalController(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate, clusterID string) error {
	logr := logger.WithFields(logrus.Fields{"updatetype": "k0s"})

	logr.Infof("Registering 'signal' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named(delegate.Name() + "-signal").
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			apsigcomm.NewSignalController(
				logr,
				mgr.GetClient(),
				delegate,
				&signalControllerHandler{
					timeout:   SignalResponseProcessingTimeout,
					clusterID: clusterID,
					k0sVersionHandler: func() (string, error) {
						return getK0sVersion(DefaultK0sStatusSocketPath)
					},
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

	sctx.Log.Infof("Found available signaling update request")

	// Confirm that the installed version of k0s requires an update
	// TODO:s0j - make the status path an argument to signaling/plan
	k0sVersion, err := h.k0sVersionHandler()
	if err != nil {
		return cr.Result{}, fmt.Errorf("unable to determine k0s version: %w", err)
	}

	sctx.Log.Infof("Current version of k0s = '%s', requested version = '%s'", k0sVersion, sctx.SignalData.Command.K0sUpdate.Version)

	// Move to 'Completed' if we match versions on a non-forced update. Otherwise, proceed to 'Downloading'
	var status = Downloading
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
