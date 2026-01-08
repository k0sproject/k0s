// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	autopilotv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	etcdv1beta1 "github.com/k0sproject/k0s/pkg/apis/etcd/v1beta1"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

func ClusterConfigs(client Provider[*k0sv1beta1.ClusterConfigList]) *Watcher[k0sv1beta1.ClusterConfig] {
	return FromClient[*k0sv1beta1.ClusterConfigList, k0sv1beta1.ClusterConfig](client)
}

func Plans(client Provider[*autopilotv1beta2.PlanList]) *Watcher[autopilotv1beta2.Plan] {
	return FromClient[*autopilotv1beta2.PlanList, autopilotv1beta2.Plan](client)
}

func ControlNodes(client Provider[*autopilotv1beta2.ControlNodeList]) *Watcher[autopilotv1beta2.ControlNode] {
	return FromClient[*autopilotv1beta2.ControlNodeList, autopilotv1beta2.ControlNode](client)
}

func Charts(client Provider[*helmv1beta1.ChartList]) *Watcher[helmv1beta1.Chart] {
	return FromClient[*helmv1beta1.ChartList, helmv1beta1.Chart](client)
}

func EtcdMembers(client Provider[*etcdv1beta1.EtcdMemberList]) *Watcher[etcdv1beta1.EtcdMember] {
	return FromClient[*etcdv1beta1.EtcdMemberList, etcdv1beta1.EtcdMember](client)
}
