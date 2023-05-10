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
	appsv1 "k8s.io/api/apps/v1"
)

func DaemonSets(client Provider[*appsv1.DaemonSetList]) *Watcher[appsv1.DaemonSet] {
	return FromClient[*appsv1.DaemonSetList, appsv1.DaemonSet](client)
}

func Deployments(client Provider[*appsv1.DeploymentList]) *Watcher[appsv1.Deployment] {
	return FromClient[*appsv1.DeploymentList, appsv1.Deployment](client)
}

func StatefulSets(client Provider[*appsv1.StatefulSetList]) *Watcher[appsv1.StatefulSet] {
	return FromClient[*appsv1.StatefulSetList, appsv1.StatefulSet](client)
}
