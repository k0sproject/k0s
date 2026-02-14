//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"
	"github.com/k0sproject/k0s/pkg/component/status"

	"github.com/sirupsen/logrus"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

const Restart = "Restart"

const (
	restartRequeueDuration = 5 * time.Second
)

// restartEventFilter creates a controller-runtime predicate that governs which
// objects will make it into reconciliation, and which will be ignored.
func restartEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(Restart),
		),
		apcomm.FalseFuncs{
			UpdateFunc: func(ue crev.UpdateEvent) bool {
				return true
			},
		},
	)
}

type restart struct {
	log      *logrus.Entry
	client   crcli.Client
	delegate apdel.ControllerDelegate
}

// registerRestart registers the 'restart' controller to the controller-runtime manager.
//
// This controller is only interested in changes to signal nodes where its signaling
// status is marked as `Restart`
func registerRestart(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	name := strings.ToLower(delegate.Name()) + "_k0s_restart"
	logger.Info("Registering reconciler: ", name)

	return cr.NewControllerManagedBy(mgr).
		Named(name).
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			&restart{
				log:      logger.WithFields(logrus.Fields{"reconciler": "k0s-restart", "object": delegate.Name()}),
				client:   mgr.GetClient(),
				delegate: delegate,
			},
		)
}

// Reconcile for the 'restart' reconciler inspects the signaling data associated to
// the provided signal node, finding if the signaling status. If the status is `Restart`,
// the `k0s` instance will be restarted.
//
// The main difference between this and the `restarted` reconciler is that this acts on
// modified events (!created), indicating that the object has actively transitioned
// to a new state.
func (r *restart) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.Name, err)
	}

	logger := r.log.WithField("signalnode", signalNode.GetName())

	// Get the current version of k0s
	logger.Info("Determining the current version of k0s")
	k0sVersion, err := getK0sVersion(status.DefaultSocketPath)
	if err != nil {
		logger.Info("Unable to determine current verion of k0s; requeuing")
		return cr.Result{}, fmt.Errorf("unable to get k0s version: %w", err)
	}

	logger.Infof("k0s version = %v is installed", k0sVersion)

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.Name, err)
	}

	if signalData.Status != nil && signalData.Status.Status != Restart {
		logger.Debug("Ignoring signal status ", signalData.Status.Status)
		return cr.Result{}, nil
	}

	if k0sVersion == signalData.Command.K0sUpdate.Version {
		signalNodeCopy := r.delegate.DeepCopy(signalNode)
		signalData.Status = apsigv2.NewStatus(UnCordoning)

		if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
			return cr.Result{}, fmt.Errorf("unable to marshal signal data for node='%s': %w", req.Name, err)
		}

		logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
		if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
			return cr.Result{}, fmt.Errorf("unable to update signal node with '%s' status: %w", signalData.Status.Status, err)
		}

		return cr.Result{}, nil
	}

	// If the found k0s version does not match the updated version, restart k0s.
	// The fact that the version of k0s was determined by the status socket, the
	// old k0s is still running.

	logger.Info("Preparing to restart k0s")

	k0sPid, err := getK0sPid(status.DefaultSocketPath)
	if err != nil {
		logger.Info("Unable to determine current k0s pid; requeuing")
		return cr.Result{RequeueAfter: restartRequeueDuration}, fmt.Errorf("unable to get k0s pid: %w", err)
	}

	// We terminate `k0s` by sending it SIGTERM. It is expected that `k0s` will be restarted
	// by some process init (systemctl, etc).

	if err := syscall.Kill(k0sPid, syscall.SIGTERM); err != nil {
		return cr.Result{}, fmt.Errorf("unable to send SIGTERM to k0s: %w", err)
	}

	return cr.Result{}, nil
}

// getK0sPid returns the PID of a running k0s based on its status socket.
func getK0sPid(statusSocketPath string) (int, error) {
	status, err := status.GetStatusInfo(statusSocketPath)
	if err != nil {
		return -1, err
	}

	return status.Pid, nil
}
