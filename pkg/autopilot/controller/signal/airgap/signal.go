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

package airgap

import (
	"context"
	"fmt"

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

// SignalControllerEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func SignalControllerEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandAirgapPredicate(),
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
}

// registerSignalController registers the 'airgap-signal' controller to the
// controller-runtime manager.
//
// This controller is only interested in changes to its own annotations, and is
// the main mechanism in identifying incoming autopilot airgap signaling
// updates.
func registerSignalController(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	logr := logger.WithFields(logrus.Fields{"updatetype": "airgap"})

	logr.Infof("Registering 'airgap-signal' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named("airgap-signal").
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			apsigcomm.NewSignalController(
				logr,
				mgr.GetClient(),
				delegate,
				&signalControllerHandler{},
			),
		)
}

// Handle will move the status to `Downloading` in order to start the airgap bundle download.
// At this time, there is no reliable version information on airgap bundles.
func (h *signalControllerHandler) Handle(ctx context.Context, sctx apsigcomm.SignalControllerContext) (cr.Result, error) {
	// A nil SignalData indicates that the request is completed, or invalid. Either way,
	// there is nothing to process.
	if sctx.SignalData == nil {
		return cr.Result{}, nil
	}

	sctx.Log.Infof("Found available signaling update request")

	// We have no way at the moment to identify what version an airgap bundle is, or what version is
	// installed, so always download.

	signalNodeCopy := sctx.Delegate.DeepCopy(sctx.SignalNode)
	status := Downloading

	sctx.SignalData.Status = apsigv2.NewStatus(status)
	if err := sctx.SignalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to marshal airgap signal data for node='%s': %w", signalNodeCopy.GetName(), err)
	}

	sctx.Log.Infof("Updating signaling response to '%s'", status)
	if err := sctx.Client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		return cr.Result{}, fmt.Errorf("unable to update airgap signal node='%s' with status='%s': %w", signalNodeCopy.GetName(), status, err)
	}

	return cr.Result{}, nil
}
