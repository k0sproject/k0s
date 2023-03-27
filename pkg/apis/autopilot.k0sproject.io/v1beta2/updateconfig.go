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

// +groupName=autopilot.k0sproject.io
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

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
// +genclient:nonNamespaced
// +groupName=autopilot.k0sproject.io
type UpdateConfig struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`

	Spec UpdateSpec `json:"spec"`
}

type UpdateSpec struct {
	Channel         string            `json:"channel,omitempty"`
	UpdateServer    string            `json:"updateServer,omitempty"`
	UpgradeStrategy UpgradeStrategy   `json:"upgradeStrategy,omitempty"`
	PlanSpec        AutopilotPlanSpec `json:"planSpec,omitempty"`
}

// AutopilotPlanSpec describes the behavior of the autopilot generated `Plan`
type AutopilotPlanSpec struct {
	// Commands are a collection of all of the commands that need to be executed
	// in order for this plan to transition to Completed.
	Commands []AutopilotPlanCommand `json:"commands"`
}

// AutopilotPlanCommand is a command that can be run within a `Plan`
type AutopilotPlanCommand struct {
	// K0sUpdate is the `K0sUpdate` command which is responsible for updating a k0s node (controller/worker)
	K0sUpdate *AutopilotPlanCommandK0sUpdate `json:"k0supdate,omitempty"`

	// AirgapUpdate is the `AirgapUpdate` command which is responsible for updating a k0s airgap bundle.
	AirgapUpdate *AutopilotPlanCommandAirgapUpdate `json:"airgapupdate,omitempty"`
}

// AutopilotPlanCommandK0sUpdate provides all of the information to for a `K0sUpdate` command to
// update a set of target signal nodes.
type AutopilotPlanCommandK0sUpdate struct {
	// ForceUpdate ensures that version checking is ignored and that all updates are applied.
	ForceUpdate bool `json:"forceupdate,omitempty"`

	// Targets defines how the controllers/workers should be discovered and upgraded.
	Targets PlanCommandTargets `json:"targets"`
}

// AutopilotPlanCommandAirgapUpdate provides all of the information to for a `AirgapUpdate` command to
// update a set of target signal nodes
type AutopilotPlanCommandAirgapUpdate struct {
	// Workers defines how the k0s workers will be discovered and airgap updated.
	Workers PlanCommandTarget `json:"workers"`
}

type UpgradeStrategy struct {
	Cron string `json:"cron"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +genclient
// +genclient:onlyVerbs=create
// +genclient:nonNamespaced
type UpdateConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UpdateConfig `json:"items"`
}
