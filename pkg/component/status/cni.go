// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	conditionCNIReady = "CNIReady"

	reasonAllReady            = "AllReady"
	reasonNotReady            = "NotReady"
	reasonNotFound            = "NotFound"
	reasonError               = "Error"
	reasonTimeout             = "Timeout"
	reasonWindowsNodesUnknown = "WindowsNodesUnknown"
	reasonNoScheduledPods     = "NoScheduledPods"
	reasonStaleGeneration     = "StaleGeneration"

	kubeRouterDaemonSet         = "kube-router"
	calicoNodeDaemonSet         = "calico-node"
	calicoWindowsDaemonSet      = "calico-node-windows"
	calicoControllersDeployment = "calico-kube-controllers"
)

func cniCondition(status corev1.ConditionStatus, reason, message string) Condition {
	return Condition{
		Type:    conditionCNIReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func apiErrorCondition(err error) Condition {
	if errors.Is(err, context.DeadlineExceeded) {
		return cniCondition(corev1.ConditionUnknown, reasonTimeout, "CNI status check timed out")
	}
	return cniCondition(corev1.ConditionUnknown, reasonError, err.Error())
}

func calicoCNICondition(ctx context.Context, client kubernetes.Interface, network *v1beta1.Network, hasWindowsNodes func() (*bool, <-chan struct{})) Condition {
	node := daemonSetCNICondition(ctx, client, calicoNodeDaemonSet)
	controllers := deploymentCNICondition(ctx, client, calicoControllersDeployment)

	parts := []Condition{node}
	required, known := windowsCalicoRequired(network, hasWindowsNodes)
	switch {
	case !known:
		parts = append(parts, cniCondition(corev1.ConditionUnknown, reasonWindowsNodesUnknown, "Windows node count is not yet known"))
	case required:
		parts = append(parts, daemonSetCNICondition(ctx, client, calicoWindowsDaemonSet))
	}
	parts = append(parts, controllers)
	return mergeCalicoConditions(parts)
}

// windowsCalicoRequired uses the Windows stack component's node counter.
// A nil counter or a nil observation means the count is not known yet.
// Status checks do not wait on the counter's changed channel.
func windowsCalicoRequired(network *v1beta1.Network, hasWindowsNodes func() (*bool, <-chan struct{})) (required, known bool) {
	if !calicoDeploysWindowsDaemonSet(network) {
		return false, true
	}
	if hasWindowsNodes == nil {
		return false, false
	}
	has, _ := hasWindowsNodes()
	if has == nil {
		return false, false
	}
	return *has, true
}

func calicoDeploysWindowsDaemonSet(network *v1beta1.Network) bool {
	mode := v1beta1.CalicoModeVXLAN
	if network != nil && network.Calico != nil && network.Calico.Mode != "" {
		mode = network.Calico.Mode
	}
	return mode == v1beta1.CalicoModeVXLAN
}

func mergeCalicoConditions(parts []Condition) Condition {
	var falseParts, unknownParts, trueParts []Condition
	for _, p := range parts {
		switch p.Status {
		case corev1.ConditionFalse:
			falseParts = append(falseParts, p)
		case corev1.ConditionUnknown:
			unknownParts = append(unknownParts, p)
		case corev1.ConditionTrue:
			trueParts = append(trueParts, p)
		default:
			unknownParts = append(unknownParts, p)
		}
	}

	if len(falseParts) > 0 {
		return cniCondition(corev1.ConditionFalse, falseParts[0].Reason, joinConditionMessages(falseParts))
	}
	if len(unknownParts) > 0 {
		return unknownParts[0]
	}
	return cniCondition(corev1.ConditionTrue, reasonAllReady, joinConditionMessages(trueParts))
}

func joinConditionMessages(parts []Condition) string {
	msgs := make([]string, 0, len(parts))
	for _, p := range parts {
		msgs = append(msgs, p.Message)
	}
	return strings.Join(msgs, "; ")
}

func daemonSetCNICondition(ctx context.Context, client kubernetes.Interface, name string) Condition {
	ds, err := client.AppsV1().DaemonSets(metav1.NamespaceSystem).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return cniCondition(corev1.ConditionFalse, reasonNotFound, name+" DaemonSet not found")
	}
	if err != nil {
		return apiErrorCondition(err)
	}

	desired := ds.Status.DesiredNumberScheduled
	available := ds.Status.NumberAvailable
	updated := ds.Status.UpdatedNumberScheduled
	message := fmt.Sprintf("%s DaemonSet reports %d/%d available, %d updated", name, available, desired, updated)
	if desired == 0 {
		return cniCondition(corev1.ConditionUnknown, reasonNoScheduledPods, message)
	}
	if ds.Status.ObservedGeneration != ds.Generation {
		return cniCondition(
			corev1.ConditionUnknown,
			reasonStaleGeneration,
			fmt.Sprintf("%s DaemonSet has not observed generation %d (observed %d)", name, ds.Generation, ds.Status.ObservedGeneration),
		)
	}
	if available != desired || updated != desired {
		return cniCondition(corev1.ConditionFalse, reasonNotReady, message)
	}
	return cniCondition(corev1.ConditionTrue, reasonAllReady, message)
}

func deploymentCNICondition(ctx context.Context, client kubernetes.Interface, name string) Condition {
	deploy, err := client.AppsV1().Deployments(metav1.NamespaceSystem).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return cniCondition(corev1.ConditionFalse, reasonNotFound, name+" Deployment not found")
	}
	if err != nil {
		return apiErrorCondition(err)
	}

	if deploy.Status.ObservedGeneration != deploy.Generation {
		return cniCondition(
			corev1.ConditionFalse,
			reasonNotReady,
			fmt.Sprintf("%s Deployment has not observed generation %d (observed %d)", name, deploy.Generation, deploy.Status.ObservedGeneration),
		)
	}

	replicas := int32(1)
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}
	updated := deploy.Status.UpdatedReplicas
	available := deploy.Status.AvailableReplicas
	total := deploy.Status.Replicas
	if replicas <= 0 || updated != replicas || available != replicas || total != updated {
		return cniCondition(
			corev1.ConditionFalse,
			reasonNotReady,
			fmt.Sprintf("%s Deployment reports %d/%d updated, %d available, %d total", name, updated, replicas, available, total),
		)
	}

	for _, c := range deploy.Status.Conditions {
		if c.Type != appsv1.DeploymentAvailable {
			continue
		}
		switch c.Status {
		case corev1.ConditionTrue:
			return cniCondition(corev1.ConditionTrue, reasonAllReady, name+" Deployment is available")
		case corev1.ConditionUnknown:
			return cniCondition(corev1.ConditionUnknown, reasonError, name+" Deployment availability is unknown")
		default:
			return cniCondition(corev1.ConditionFalse, reasonNotReady, name+" Deployment is not available")
		}
	}
	return cniCondition(corev1.ConditionFalse, reasonNotReady, name+" Deployment has no Available condition")
}
