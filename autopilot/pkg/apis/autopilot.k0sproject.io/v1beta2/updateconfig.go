// Copyright 2022 k0s authors
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

//+groupName=autopilot.k0sproject.io
package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(
		&UpdateConfig{},
		&UpdateConfigList{},
	)
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+genclient
//+genclient:onlyVerbs=create,delete,list,get,watch,update
//+genclient:nonNamespaced
// +groupName=autopilot.k0sproject.io
type UpdateConfig struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`

	Spec UpdateSpec `json:"spec"`
}

type UpdateSpec struct {
	Channel         string          `json:"channel,omitempty"`
	UpdateServer    string          `json:"updateServer,omitempty"`
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy,omitempty"`
	PlanSpec        PlanSpec        `json:"planSpec,omitempty"`
}

type UpgradeStrategy struct {
	Cron string `json:"cron"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+genclient
//+genclient:onlyVerbs=create
//+genclient:nonNamespaced
type UpdateConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UpdateConfig `json:"items"`
}
