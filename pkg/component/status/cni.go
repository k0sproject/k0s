// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

const (
	conditionCNIReady = "CNIReady"

	reasonAllReady           = "AllReady"
	reasonNotReady           = "NotReady"
	reasonComponentsNotReady = "ComponentsNotReady"
	reasonNotFound           = "NotFound"
	reasonError              = "Error"
	reasonTimeout            = "Timeout"

	kubeRouterDaemonSet         = "kube-router"
	calicoNodeDaemonSet         = "calico-node"
	calicoWindowsDaemonSet      = "calico-node-windows"
	calicoControllersDeployment = "calico-kube-controllers"
)

// cniCheckTimeout bounds live API reads so a stuck apiserver cannot hang
// `k0s status` on the unix socket.
var cniCheckTimeout = 5 * time.Second

func cniCondition(status corev1.ConditionStatus, reason, message string) Condition {
	return Condition{
		Type:    conditionCNIReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func apiErrorCondition(err error) Condition {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return cniCondition(corev1.ConditionUnknown, reasonTimeout, "CNI status check timed out")
	}
	return cniCondition(corev1.ConditionUnknown, reasonError, err.Error())
}

// evaluateCNI reports managed CNI health from DaemonSet and Deployment status.
// A CNI conf file on disk is not used: kubelet treats that file as enough to
// mark the node Ready even when the CNI is not operational.
// Custom CNI is skipped (ok=false). Callers must pass a live API client.
func evaluateCNI(ctx context.Context, client kubernetes.Interface, network *v1beta1.Network) (Condition, bool) {
	if network == nil {
		return Condition{}, false
	}
	switch network.Provider {
	case constant.CNIProviderKubeRouter:
		return daemonSetCNICondition(ctx, client, kubeRouterDaemonSet), true
	case constant.CNIProviderCalico:
		return calicoCNICondition(ctx, client, network), true
	default:
		return Condition{}, false
	}
}

func calicoCNICondition(ctx context.Context, client kubernetes.Interface, network *v1beta1.Network) Condition {
	node := daemonSetCNICondition(ctx, client, calicoNodeDaemonSet)
	controllers := deploymentCNICondition(ctx, client, calicoControllersDeployment)
	if controllers.Status == corev1.ConditionFalse && controllers.Reason == reasonNotReady {
		controllers.Reason = reasonComponentsNotReady
	}

	parts := []Condition{node}
	requireWindows, err := windowsCalicoRequired(ctx, client, network)
	if err != nil {
		parts = append(parts, apiErrorCondition(err))
	} else if requireWindows {
		parts = append(parts, daemonSetCNICondition(ctx, client, calicoWindowsDaemonSet))
	}
	parts = append(parts, controllers)
	return mergeCalicoConditions(parts)
}

func windowsCalicoRequired(ctx context.Context, client kubernetes.Interface, network *v1beta1.Network) (bool, error) {
	if !calicoDeploysWindowsDaemonSet(network) {
		return false, nil
	}
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fields.OneTermEqualSelector(corev1.LabelOSStable, string(corev1.Windows)).String(),
		Limit:         1,
	})
	if err != nil {
		return false, err
	}
	return len(nodes.Items) > 0, nil
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
		default:
			trueParts = append(trueParts, p)
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

	if ds.Status.ObservedGeneration != ds.Generation {
		return cniCondition(
			corev1.ConditionFalse,
			reasonNotReady,
			fmt.Sprintf("%s DaemonSet has not observed generation %d (observed %d)", name, ds.Generation, ds.Status.ObservedGeneration),
		)
	}

	desired := ds.Status.DesiredNumberScheduled
	available := ds.Status.NumberAvailable
	updated := ds.Status.UpdatedNumberScheduled
	message := fmt.Sprintf("%s DaemonSet reports %d/%d available, %d updated", name, available, desired, updated)
	if desired <= 0 || available != desired || updated != desired {
		return cniCondition(corev1.ConditionFalse, reasonNotReady, message)
	}

	currentReady, err := countReadyCurrentDaemonPods(ctx, client, ds)
	if err != nil {
		return apiErrorCondition(err)
	}
	if currentReady < desired {
		return cniCondition(
			corev1.ConditionFalse,
			reasonNotReady,
			fmt.Sprintf("%s DaemonSet has %d/%d current-template pods ready", name, currentReady, desired),
		)
	}
	return cniCondition(corev1.ConditionTrue, reasonAllReady, message)
}

func countReadyCurrentDaemonPods(ctx context.Context, client kubernetes.Interface, ds *appsv1.DaemonSet) (int32, error) {
	hash, err := currentDaemonSetRevisionHash(ctx, client, ds)
	if err != nil {
		return 0, err
	}
	if hash == "" || ds.Spec.Selector == nil {
		return 0, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return 0, err
	}
	pods, err := client.CoreV1().Pods(ds.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return 0, err
	}
	var ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !ownedByDaemonSet(pod, ds) {
			continue
		}
		if pod.Labels[appsv1.DefaultDaemonSetUniqueLabelKey] != hash {
			continue
		}
		if podReady(pod) {
			ready++
		}
	}
	return ready, nil
}

func currentDaemonSetRevisionHash(ctx context.Context, client kubernetes.Interface, ds *appsv1.DaemonSet) (string, error) {
	if ds.Spec.Selector == nil {
		return "", nil
	}
	selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return "", err
	}
	revs, err := client.AppsV1().ControllerRevisions(ds.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", err
	}
	var current *appsv1.ControllerRevision
	for i := range revs.Items {
		rev := &revs.Items[i]
		if !ownedByDaemonSet(rev, ds) {
			continue
		}
		if current == nil || rev.Revision > current.Revision {
			current = rev
		}
	}
	if current == nil {
		return "", nil
	}
	if hash := current.Labels[appsv1.DefaultDaemonSetUniqueLabelKey]; hash != "" {
		return hash, nil
	}
	return current.Labels[appsv1.ControllerRevisionHashLabelKey], nil
}

func ownedByDaemonSet(obj metav1.Object, ds *appsv1.DaemonSet) bool {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "DaemonSet" && owner.Name == ds.Name {
			return true
		}
	}
	return false
}

func podReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
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
