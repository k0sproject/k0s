// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
)

func ConfigMaps(client Provider[*corev1.ConfigMapList]) *Watcher[corev1.ConfigMap] {
	return FromClient[*corev1.ConfigMapList, corev1.ConfigMap](client)
}

func EndpointSlices(client Provider[*discoveryv1.EndpointSliceList]) *Watcher[discoveryv1.EndpointSlice] {
	return FromClient[*discoveryv1.EndpointSliceList, discoveryv1.EndpointSlice](client)
}

func Nodes(client Provider[*corev1.NodeList]) *Watcher[corev1.Node] {
	return FromClient[*corev1.NodeList, corev1.Node](client)
}

func Pods(client Provider[*corev1.PodList]) *Watcher[corev1.Pod] {
	return FromClient[*corev1.PodList, corev1.Pod](client)
}

func Events(client Provider[*corev1.EventList]) *Watcher[corev1.Event] {
	return FromClient[*corev1.EventList, corev1.Event](client)
}
