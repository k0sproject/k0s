//go:build unix

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

package k0s

import (
	"context"
	"fmt"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

type restarted struct {
	log      *logrus.Entry
	client   crcli.Client
	delegate apdel.ControllerDelegate
}

// restartedEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func restartedEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(Restart),
		),
		apcomm.FalseFuncs{
			CreateFunc: func(ce crev.CreateEvent) bool {
				return true
			},
		},
	)
}

// registerRestarted registers the 'restart' controller to the controller-runtime manager.
//
// This controller is only interested in changes to signal nodes where its signaling
// status is marked as `Restart`
func registerRestarted(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	logger.Infof("Registering 'restarted' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named(delegate.Name() + "-restarted").
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			&restarted{
				log:      logger.WithFields(logrus.Fields{"reconciler": "restarted", "object": delegate.Name()}),
				client:   mgr.GetClient(),
				delegate: delegate,
			},
		)
}

// Reconcile for the 'restarted' reconciler inspects the signaling data associated to
// the provided signal node, finding if the signaling status. If the status is `Restart`,
// the `k0s` instance will be restarted.
//
// The main difference between this and the `restart` reconciler is that this triggers
// when the event is "created", indicating that `k0s` has actually restarted.
//
// If the installed `k0s` version is the version specified in the plan (or if a `forceupdate`),
// the plan will move to 'Completed'.
func (r *restarted) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.NamespacedName.Name, err)
	}

	logger := r.log.WithField("signalnode", signalNode.GetName())

	// Get the current version of k0s
	logger.Info("Determining the current version of k0s")
	k0sVersion, err := getK0sVersion(DefaultK0sStatusSocketPath)
	if err != nil {
		logger.Info("Unable to determine current verion of k0s; requeuing")
		return cr.Result{}, fmt.Errorf("unable to get k0s version: %w", err)
	}

	logger.Infof("k0s version = %v is installed", k0sVersion)

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.NamespacedName.Name, err)
	}

	// Move to the next successful state 'UnCordoning' if our versions match

	if k0sVersion == signalData.Command.K0sUpdate.Version || signalData.Command.K0sUpdate.ForceUpdate {
		signalNodeCopy := r.delegate.DeepCopy(signalNode)
		signalData.Status = apsigv2.NewStatus(UnCordoning)

		if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
			return cr.Result{}, fmt.Errorf("unable to marshal signal data for node='%s': %w", req.NamespacedName.Name, err)
		}

		logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
		if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
			return cr.Result{}, fmt.Errorf("unable to update signal node with '%s' status: %w", signalData.Status.Status, err)
		}

		return cr.Result{}, nil
	}

	return cr.Result{}, nil
}
