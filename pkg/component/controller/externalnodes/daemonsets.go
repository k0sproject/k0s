// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package externalnodes

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// With node-local load balancing, k0s points kube-proxy and konnectivity-agent at localhost.
// Nodes without k0s don't run those load balancers, so they get copies that use the real endpoints.
const (
	kubeProxyName    = "kube-proxy"
	konnectivityName = "konnectivity-agent"
	copySuffix       = "-external"
	kubeProxyConfKey = "kubeconfig.conf"
	appLabel         = "k8s-app"
)

// kubeProxyCopy derives the kube-proxy ConfigMap and DaemonSet for nodes without k0s.
func kubeProxyCopy(cm *corev1.ConfigMap, ds *appsv1.DaemonSet, serverURL string) (*corev1.ConfigMap, *appsv1.DaemonSet, error) {
	kubeconfig, err := clientcmd.Load([]byte(cm.Data[kubeProxyConfKey]))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load the kube-proxy kubeconfig: %w", err)
	}
	if len(kubeconfig.Clusters) == 0 {
		return nil, nil, errors.New("the kube-proxy kubeconfig has no clusters")
	}
	for _, cluster := range kubeconfig.Clusters {
		cluster.Server = serverURL
	}
	data, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	cmCopy := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: copyMeta(&cm.ObjectMeta, kubeProxyName+copySuffix),
		Data:       maps.Clone(cm.Data),
	}
	cmCopy.Data[kubeProxyConfKey] = string(data)

	dsCopy := daemonSetCopy(ds, kubeProxyName+copySuffix)
	for i := range dsCopy.Spec.Template.Spec.Volumes {
		if v := &dsCopy.Spec.Template.Spec.Volumes[i]; v.ConfigMap != nil && v.ConfigMap.Name == cm.Name {
			v.ConfigMap.Name = cmCopy.Name
		}
	}
	return cmCopy, dsCopy, nil
}

// konnectivityCopy derives the konnectivity-agent DaemonSet for nodes without k0s.
func konnectivityCopy(ds *appsv1.DaemonSet, host string, port int32) *appsv1.DaemonSet {
	dsCopy := daemonSetCopy(ds, konnectivityName+copySuffix)
	pod := &dsCopy.Spec.Template.Spec
	// The agents only need the host network to reach the node-local load balancers.
	pod.HostNetwork = false
	if pod.SecurityContext != nil {
		pod.SecurityContext.WindowsOptions = nil
	}
	for i := range pod.Containers {
		for j, arg := range pod.Containers[i].Args {
			switch {
			case strings.HasPrefix(arg, "--proxy-server-host="):
				pod.Containers[i].Args[j] = "--proxy-server-host=" + host
			case strings.HasPrefix(arg, "--proxy-server-port="):
				pod.Containers[i].Args[j] = "--proxy-server-port=" + strconv.FormatInt(int64(port), 10)
			}
		}
	}
	return dsCopy
}

// daemonSetCopy copies the DaemonSet under a new name, with its own selector, scheduled on nodes without k0s only.
func daemonSetCopy(ds *appsv1.DaemonSet, name string) *appsv1.DaemonSet {
	dsCopy := &appsv1.DaemonSet{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "DaemonSet"},
		ObjectMeta: copyMeta(&ds.ObjectMeta, name),
		Spec:       *ds.Spec.DeepCopy(),
	}
	dsCopy.Labels[appLabel] = name
	dsCopy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{appLabel: name}}
	tmpl := &dsCopy.Spec.Template
	if tmpl.Labels == nil {
		tmpl.Labels = map[string]string{}
	}
	tmpl.Labels[appLabel] = name
	tmpl.Spec.Affinity = requireNode(tmpl.Spec.Affinity, corev1.NodeSelectorOpIn)
	return dsCopy
}

// requireNode sets a requirement on the k0s label being false, or not false, in each node selector term.
// It replaces the requirement of the original, which keeps off nodes without k0s.
func requireNode(affinity *corev1.Affinity, op corev1.NodeSelectorOperator) *corev1.Affinity {
	requirement := corev1.NodeSelectorRequirement{Key: constant.K0sNodeLabel, Operator: op, Values: []string{"false"}}
	if affinity == nil {
		affinity = &corev1.Affinity{}
	}
	if affinity.NodeAffinity == nil {
		affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	required := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if required == nil || len(required.NodeSelectorTerms) == 0 {
		required = &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{}}}
	}
	for i := range required.NodeSelectorTerms {
		term := &required.NodeSelectorTerms[i]
		term.MatchExpressions = slices.DeleteFunc(term.MatchExpressions, func(r corev1.NodeSelectorRequirement) bool {
			return r.Key == constant.K0sNodeLabel
		})
		term.MatchExpressions = append(term.MatchExpressions, requirement)
	}
	affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = required
	return affinity
}

// copyMeta keeps the labels and annotations of the original, without the ones of its stack.
func copyMeta(meta *metav1.ObjectMeta, name string) metav1.ObjectMeta {
	labels := maps.Clone(meta.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	delete(labels, applier.NameLabel)
	annotations := maps.Clone(meta.Annotations)
	for _, key := range []string{applier.ChecksumAnnotation, applier.LastConfigAnnotation, appsv1.DeprecatedTemplateGeneration} {
		delete(annotations, key)
	}
	return metav1.ObjectMeta{Namespace: meta.Namespace, Name: name, Labels: labels, Annotations: annotations}
}
