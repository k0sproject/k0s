//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"strings"

	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
	cr "sigs.k8s.io/controller-runtime"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

const UnCordoning = "UnCordoning"

// unCordoningEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func unCordoningEventFilter(handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(UnCordoning),
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

// registerUncordoning registers the 'uncordoning' controller to the
// controller-runtime manager.
//
// This controller is only interested when autopilot signaling annotations have
// moved to a `Cordoning` status. At this point, it will attempt to cordong & drain
// the node.
func registerUncordoning(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate, nodeName types.NodeName, leaseStatus leaderelection.Status) error {
	name := strings.ToLower(delegate.Name()) + "_k0s_uncordoning"
	logger.Info("Registering reconciler: ", name)

	// create the clientset
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	return cr.NewControllerManagedBy(mgr).
		Named(name).
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			&cordonUncordon{
				log:          logger.WithFields(logrus.Fields{"reconciler": "k0s-uncordoning", "object": delegate.Name()}),
				client:       mgr.GetClient(),
				delegate:     delegate,
				clientset:    clientset,
				currentState: UnCordoning,
				nodeName:     nodeName,
				leaseStatus:  leaseStatus,
				do:           uncordonNode,
				nextState:    apsigcomm.Completed,
			},
		)
}

func uncordonNode(drainer *drain.Helper, node *corev1.Node) error {
	return drain.RunCordonOrUncordon(drainer, node, false)
}
