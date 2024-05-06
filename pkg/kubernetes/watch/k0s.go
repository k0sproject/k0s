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

func Charts(client Provider[*helmv1beta1.ChartList]) *Watcher[helmv1beta1.Chart] {
	return FromClient[*helmv1beta1.ChartList, helmv1beta1.Chart](client)
}

func EtcdMembers(client Provider[*etcdv1beta1.EtcdMemberList]) *Watcher[etcdv1beta1.EtcdMember] {
	return FromClient[*etcdv1beta1.EtcdMemberList, etcdv1beta1.EtcdMember](client)
}
