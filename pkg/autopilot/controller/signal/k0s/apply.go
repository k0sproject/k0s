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
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apconst "github.com/k0sproject/k0s/pkg/autopilot/constant"
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

// applyingUpdateEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func applyingUpdateEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(ApplyingUpdate),
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

type applyingUpdate struct {
	log          *logrus.Entry
	client       crcli.Client
	delegate     apdel.ControllerDelegate
	k0sBinaryDir string
}

// registeryApplyingUpdate registers the 'applying-update' controller to the
// controller-runtime manager.
//
// This controller is only interested in taking the downloaded update, and
// applying it to the current k0s install.
func registerApplyingUpdate(
	logger *logrus.Entry,
	mgr crman.Manager,
	eventFilter crpred.Predicate,
	delegate apdel.ControllerDelegate,
	k0sBinaryDir string,
) error {
	logger.Infof("Registering 'applying-update' reconciler for '%s'", delegate.Name())

	return cr.NewControllerManagedBy(mgr).
		Named(delegate.Name() + "-applying-update").
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			&applyingUpdate{
				log:          logger.WithFields(logrus.Fields{"reconciler": "applying-update", "object": delegate.Name()}),
				client:       mgr.GetClient(),
				delegate:     delegate,
				k0sBinaryDir: k0sBinaryDir,
			},
		)
}

// Reconcile for the 'applying-update' reconciler will attempt to apply the update
// over the existing k0s installation. This involves permission updates, moving,
// and restarting the k0s service.
func (r *applyingUpdate) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.NamespacedName.Name, err)
	}

	logger := r.log.WithField("signalnode", signalNode.GetName())
	logger.Info("Applying update")

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.NamespacedName.Name, err)
	}

	updateFilenamePath := path.Join(r.k0sBinaryDir, apconst.K0sTempFilename)

	// Ensure that the expected file exists
	if _, err := os.Stat(updateFilenamePath); errors.Is(err, os.ErrNotExist) {
		return cr.Result{}, fmt.Errorf("unable to find update file '%s': %w", apconst.K0sTempFilename, err)
	}

	// Ensure that the new file is executable
	if err := os.Chmod(updateFilenamePath, 0755); err != nil {
		return cr.Result{}, fmt.Errorf("unable to chmod update file '%s': %w", apconst.K0sTempFilename, err)
	}

	// Perform the update atomically
	if err := os.Rename(updateFilenamePath, filepath.Join(r.k0sBinaryDir, "k0s")); err != nil {
		return cr.Result{}, fmt.Errorf("unable to update (rename) to the new file: %w", err)
	}

	// When the k0s process has been terminated, move to 'Restart'
	signalNodeCopy := r.delegate.DeepCopy(signalNode)

	signalData.Status = apsigv2.NewStatus(Restart)
	if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to marshal signal data for node='%s': %w", req.NamespacedName.Name, err)
	}

	logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
	if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		return cr.Result{Requeue: true}, fmt.Errorf("failed to update signal node to status '%s': %w", signalData.Status.Status, err)
	}

	return cr.Result{}, nil
}
