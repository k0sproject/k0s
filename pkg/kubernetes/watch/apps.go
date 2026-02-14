// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
