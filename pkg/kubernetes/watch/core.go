/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package watch

import (
	corev1 "k8s.io/api/core/v1"
)

func ConfigMaps(client Provider[*corev1.ConfigMapList]) *Watcher[corev1.ConfigMap] {
	return FromClient[*corev1.ConfigMapList, corev1.ConfigMap](client)
}

func Endpoints(client Provider[*corev1.EndpointsList]) *Watcher[corev1.Endpoints] {
	return FromClient[*corev1.EndpointsList, corev1.Endpoints](client)
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
