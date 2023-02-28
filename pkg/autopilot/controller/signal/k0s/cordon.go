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
	apsigpred "github.com/k0sproject/k0s/pkg/autopilot/controller/signal/common/predicate"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	cr "sigs.k8s.io/controller-runtime"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	crev "sigs.k8s.io/controller-runtime/pkg/event"
	crman "sigs.k8s.io/controller-runtime/pkg/manager"
	crpred "sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

// cordoningEventFilter creates a controller-runtime predicate that governs which objects
// will make it into reconciliation, and which will be ignored.
func cordoningEventFilter(hostname string, handler apsigpred.ErrorHandler) crpred.Predicate {
	return crpred.And(
		crpred.AnnotationChangedPredicate{},
		apsigpred.SignalNamePredicate(hostname),
		apsigpred.NewSignalDataPredicateAdapter(handler).And(
			signalDataUpdateCommandK0sPredicate(),
			apsigpred.SignalDataStatusPredicate(Cordoning),
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

type cordoning struct {
	log       *logrus.Entry
	client    crcli.Client
	delegate  apdel.ControllerDelegate
	clientset *kubernetes.Clientset
}

// registerCordoning registers the 'cordoning' controller to the
// controller-runtime manager.
//
// This controller is only interested when autopilot signaling annotations have
// moved to a `Cordoning` status. At this point, it will attempt to cordong & drain
// the node.
func registerCordoning(logger *logrus.Entry, mgr crman.Manager, eventFilter crpred.Predicate, delegate apdel.ControllerDelegate) error {
	logger.Infof("Registering 'cordoning' reconciler for '%s'", delegate.Name())

	// create the clientset
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	return cr.NewControllerManagedBy(mgr).
		For(delegate.CreateObject()).
		WithEventFilter(eventFilter).
		Complete(
			&cordoning{
				log:       logger.WithFields(logrus.Fields{"reconciler": "cordoning", "object": delegate.Name()}),
				client:    mgr.GetClient(),
				delegate:  delegate,
				clientset: clientset,
			},
		)
}

// Reconcile for the 'cordoning' reconciler will cordon and drain a node
func (r *cordoning) Reconcile(ctx context.Context, req cr.Request) (cr.Result, error) {
	signalNode := r.delegate.CreateObject()
	if err := r.client.Get(ctx, req.NamespacedName, signalNode); err != nil {
		return cr.Result{}, fmt.Errorf("unable to get signal for node='%s': %w", req.NamespacedName.Name, err)
	}

	logger := r.log.WithField("signalnode", signalNode.GetName())

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNode.GetAnnotations()); err != nil {
		return cr.Result{}, fmt.Errorf("unable to unmarshal signal data for node='%s': %w", req.NamespacedName.Name, err)
	}
	if !needsCordoning(signalNode) {
		logger.Infof("ignoring non worker node")

		return cr.Result{}, r.moveToNextState(ctx, signalNode, ApplyingUpdate)
	}

	logger.Infof("starting to cordon node %s", signalNode.GetName())
	if err := r.drainNode(ctx, signalNode); err != nil {
		return cr.Result{}, err
	}

	return cr.Result{}, r.moveToNextState(ctx, signalNode, ApplyingUpdate)
}

func (r *cordoning) moveToNextState(ctx context.Context, signalNode crcli.Object, state string) error {
	logger := r.log.WithField("signalnode", signalNode.GetName())

	signalNodeCopy := r.delegate.DeepCopy(signalNode)

	var signalData apsigv2.SignalData
	if err := signalData.Unmarshal(signalNodeCopy.GetAnnotations()); err != nil {
		return fmt.Errorf("unable to unmarshal signal data: %w", err)
	}

	signalData.Status = apsigv2.NewStatus(state)

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

// drainNode cordons a node after which drains it
// draining ignores daemonsets
func (r *cordoning) drainNode(ctx context.Context, signalNode crcli.Object) error {
	logger := r.log.WithField("signalnode", signalNode.GetName()).WithField("phase", "drain")

	node := &corev1.Node{}
	// if signalNode is a Node cast it to *corev1.Node
	if signalNode.GetObjectKind().GroupVersionKind().Kind == "Node" {
		var ok bool
		node, ok = signalNode.(*corev1.Node)
		if !ok {
			return fmt.Errorf("failed to cast signalNode to *corev1.Node")
		}
	} else {
		//otherwise get node from client
		if err := r.client.Get(ctx, crcli.ObjectKey{Name: signalNode.GetName()}, node); err != nil {
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
		OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
			logger.Infof("evicted pod: %s/%s", pod.Namespace, pod.Name)
		},
	}

	if err := drain.RunCordonOrUncordon(drainer, node, true); err != nil {
		return err
	}

	if err := drain.RunNodeDrain(drainer, node.Name); err != nil {
		return err
	}

	return nil
}
