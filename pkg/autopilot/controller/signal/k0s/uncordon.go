//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package k0s

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apcomm "github.com/k0sproject/k0s/pkg/autopilot/common"
	apdel "github.com/k0sproject/k0s/pkg/autopilot/controller/delegate"
	apsigcomm "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common"
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"
)

const UnCordoning = "UnCordoning"

// unCordoningEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func unCordoningEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
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

type uncordoning struct {
	log       *logrus.Entry
	client    crcli.Client
	delegate  apdel.ControllerDelegate
	clientset *kubernetes.Clientset
}

// registerUncordoning registers the 'uncordoning' controller to the
// controller-runtime manager.
//
// This controller is only interested when autopilot signaling annotations have
// moved to a `Cordoning` status. At this point, it will attempt to cordong & drain
// the node.
func registerUncordoning(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
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
			&uncordoning{
				log:       logger.WithFields(logrus.Fields{"reconciler": "k0s-uncordoning", "object": delegate.Name()}),
				client:    mgr.GetClient(),
				delegate:  delegate,
				clientset: clientset,
			},
		)
}

// Reconcile for the 'cordoning' reconciler will cordon and drain a node
func (r *uncordoning) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.Name, err)
	}

	logger := r.log.WithField("signalnode", signalNode.GetName())

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.Name, err)
	}

	if !needsCordoning(signalNode) {
		logger.Infof("ignoring non worker node")

		return cr.Result{}, r.moveToNextState(ctx, signalNode, apsigcomm.Completed)
	}

	logger.Infof("starting to un-cordon node %s", signalNode.GetName())
	if err := r.unCordonNode(ctx, signalNode); err != nil {
		return cr.Result{}, err
	}

	return cr.Result{}, r.moveToNextState(ctx, signalNode, apsigcomm.Completed)
}

func (r *uncordoning) moveToNextState(ctx context.Context, signalNode crcli.Object, state string) error {
	logger := r.log.WithField("signalnode", signalNode.GetName())

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return fmt.Errorf("unable to unmarshal signal data: %w", err)
	}

	signalData.Status = apsigv2.NewStatus(state)
	signalNodeCopy := r.delegate.DeepCopy(signalNode)

	if err := signalData.Marshal(signalNodeCopy.GetAnnotations()); err != nil {
		return fmt.Errorf("unable to marshal signal data: %w", err)
	}

	logger.Infof("Updating signaling response to '%s'", signalData.Status.Status)
	if err := r.client.Update(ctx, signalNodeCopy, &crcli.UpdateOptions{}); err != nil {
		logger.Errorf("Failed to update signal node to status '%s': %v", signalData.Status.Status, err)
		return err
	}
	return nil
}

// unCordonNode un-cordons a node
func (r *uncordoning) unCordonNode(ctx context.Context, signalNode crcli.Object) error {
	logger := r.log.WithField("signalnode", signalNode.GetName()).WithField("phase", "uncordon")

	node := &corev1.Node{}

	// if signalNode is a Node cast it to *corev1.Node
	if signalNode.GetObjectKind().GroupVersionKind().Kind == "Node" {
		var ok bool
		node, ok = signalNode.(*corev1.Node)
		if !ok {
			return errors.New("failed to convert signalNode to Node")
		}
	} else {
		nodeName := signalNode.GetName()
		controlNode, ok := signalNode.(*autopilotv1beta2.ControlNode)
		if ok {
			for _, addr := range controlNode.Status.Addresses {
				if addr.Type == corev1.NodeHostName {
					nodeName = addr.Address
					break
				}
			}
		}

		// otherwise get node from client
		if err := r.client.Get(ctx, crcli.ObjectKey{Name: nodeName}, node); err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}
	}

	drainer := &drain.Helper{
		Client: r.clientset,
		Force:  true,
		// negative value to use the pod's terminationGracePeriodSeconds
		GracePeriodSeconds:  -1,
		IgnoreAllDaemonSets: true,
		Ctx:                 ctx,
		Out:                 logger.Writer(),
		ErrOut:              logger.Writer(),
		// We want to proceed even when pods are using emptyDir volumes
		DeleteEmptyDirData: true,
		Timeout:            time.Duration(120) * time.Second,
	}

	if err := drain.RunCordonOrUncordon(drainer, node, false); err != nil {
		return err
	}

	return nil
}
