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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControlNode is a node which behaves as a controller, able to receive autopilot
// signaling updates.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update,updateStatus
// +genclient:nonNamespaced
type ControlNode struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`
	Status            ControlNodeStatus `json:"status,omitempty"`
}

// ControlNodeStatus has the runtime status info of the controller such as address etc.
type ControlNodeStatus struct {
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`
}

// GetInternalIP returns the internal IP address for the object. Returns empty string if the object does not have InternalIP set.
func (c *ControlNodeStatus) GetInternalIP() string {
	for _, addr := range c.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

// ControlNodeList is a list of ControlNode instances.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type ControlNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ControlNode `json:"items"`
}

// Plan provides all details of what to execute as a part of the plan, and
// the current status of its execution.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
// +genclient:nonNamespaced
type Plan struct {
	metav1.TypeMeta `json:",omitempty,inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines how the plan behaves.
	Spec PlanSpec `json:"spec"`

	// Status is the most recently observed status of the plan.
	Status PlanStatus `json:"status,omitempty"`
}

// PlanList is a list of Plan instances.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type PlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Plan `json:"items"`
}

// PlanSpec describes the behavior of the `Plan`
type PlanSpec struct {
	// ID is a user-provided identifier for this plan.
	ID string `json:"id"`

	// Timestamp is a user-provided time that the plan was created.
	Timestamp string `json:"timestamp"`

	// Commands are a collection of all of the commands that need to be executed
	// in order for this plan to transition to Completed.
	Commands []PlanCommand `json:"commands"`
}

// PlanCommand is a command that can be run within a `Plan`
type PlanCommand struct {
	// K0sUpdate is the `K0sUpdate` command which is responsible for updating a k0s node (controller/worker)
	K0sUpdate *PlanCommandK0sUpdate `json:"k0supdate,omitempty"`

	// AirgapUpdate is the `AirgapUpdate` command which is responsible for updating a k0s airgap bundle.
	AirgapUpdate *PlanCommandAirgapUpdate `json:"airgapupdate,omitempty"`
}

// PlanPlatformResourceURLMap is a mapping of `PlanResourceURL` instances mapped to platform identifiers.
type PlanPlatformResourceURLMap map[string]PlanResourceURL

// PlanCommandK0sUpdate provides all of the information to for a `K0sUpdate` command to
// update a set of target signal nodes.
type PlanCommandK0sUpdate struct {
	// Version is the version that `K0sUpdate` will be upgrading to.
	Version string `json:"version"`

	// ForceUpdate ensures that version checking is ignored and that all updates are applied.
	ForceUpdate bool `json:"forceupdate,omitempty"`

	// Platforms is a map of PlanResourceUrls to platform identifiers, allowing a single k0s version
	// to have multiple URL resources based on platform.
	Platforms PlanPlatformResourceURLMap `json:"platforms"`

	// Targets defines how the controllers/workers should be discovered and upgraded.
	Targets PlanCommandTargets `json:"targets"`
}

// PlanCommandAirgapUpdate provides all of the information to for a `AirgapUpdate` command to
// update a set of target signal nodes
type PlanCommandAirgapUpdate struct {
	// Version is the version that `AirgapUpdate` will be upgrading to.
	Version string `json:"version"`

	// Platforms is a map of PlanResourceUrls to platform identifiers, allowing a single k0s airgap
	// version to have multiple Url resources based on platform.
	Platforms PlanPlatformResourceURLMap `json:"platforms"`

	// Workers defines how the k0s workers will be discovered and airgap updated.
	Workers PlanCommandTarget `json:"workers"`
}

// PlanResourceURL is a remote URL resource.
type PlanResourceURL struct {
	// URL is the URL of a downloadable resource.
	URL string `json:"url"`

	// Sha256 provides an optional SHA256 hash of the URL's content for verification.
	Sha256 string `json:"sha256,omitempty"`
}

// PlanCommandTargets contains the target definitions for both controllers and workers.
type PlanCommandTargets struct {
	// Controllers defines how k0s controllers will be discovered and executed.
	Controllers PlanCommandTarget `json:"controllers,omitempty"`

	// Workers defines how k0s workers will be discovered and executed.
	Workers PlanCommandTarget `json:"workers,omitempty"`
}

// PlanCommandTarget defines how a plan should discover signal nodes that should be considered
// grouped into this target, along with any limitations that should be imposed.
type PlanCommandTarget struct {
	// Discovery details how nodes for this target should be discovered.
	Discovery PlanCommandTargetDiscovery `json:"discovery"`

	// Limits impose various limits and restrictions on how discovery and execution should behave.
	//
	// +kubebuilder:default={concurrent:1}
	Limits PlanCommandTargetLimits `json:"limits,omitempty"`
}

// PlanCommandTargetLimits are limits that can be imposed on a target of a command.
type PlanCommandTargetLimits struct {
	// Concurrent specifies the number of concurrent target executions that can be performed
	// within this target. (ie. '2' == at most have 2 execute at the same time)
	//
	// +kubebuilder:default=1
	Concurrent int `json:"concurrent,omitempty"`
}

// PlanCommandTargetDiscovery contains the type of discovery mechanism that should be used
// for resolving signal nodes.
type PlanCommandTargetDiscovery struct {
	// Static provides a static means of identifying target signal nodes.
	Static *PlanCommandTargetDiscoveryStatic `json:"static,omitempty"`

	// Selector provides a kubernetes 'selector' means of identifying target signal nodes.
	Selector *PlanCommandTargetDiscoverySelector `json:"selector,omitempty"`
}

// PlanCommandTargetDiscoveryStatic is a discovery mechanism for resolving signal nodes
// using a predefined static set of nodes.
type PlanCommandTargetDiscoveryStatic struct {
	// Nodes provides a static set of target signal nodes.
	Nodes []string `json:"nodes,omitempty"`
}

// PlanCommandTargetDiscoverySelector is a discovery mechanism for resolving signal nodes
// using standard Kubernetes 'Field' and 'Label' selectors.
type PlanCommandTargetDiscoverySelector struct {
	// Labels is a standard kubernetes label selector (key=value,key=value,...)
	Labels string `json:"labels,omitempty"`

	// Fields is a standard kubernetes field selector (key=value,key=value,...)
	Fields string `json:"fields,omitempty"`
}

// PlanStateType is the state of a Plan
type PlanStateType string

// PlanStatus contains the status and state of the entire plan operation.
type PlanStatus struct {
	// State is the current state of the plan. This value typically mirrors the status
	// of the current command execution to allow for querying a single field to determine
	// the plan status.
	State PlanStateType `json:"state"`

	// Commands are a collection of status's for each of the commands defined in the plan,
	// maintained in their index order.
	Commands []PlanCommandStatus `json:"commands"`
}

// PlanCommandStatus is the status of a known command.
type PlanCommandStatus struct {
	// ID is a unique identifier for this command in a Plan
	ID int `json:"id"`

	// State is the current state of the plan command.
	State PlanStateType `json:"state"`

	// Description is the additional information about the plan command state.
	Description string `json:"description,omitempty"`

	// K0sUpdate is the status of the `K0sUpdate` command.
	K0sUpdate *PlanCommandK0sUpdateStatus `json:"k0supdate,omitempty"`

	// AirgapUpdate is the status of the `AirgapUpdate` command.
	AirgapUpdate *PlanCommandAirgapUpdateStatus `json:"airgapupdate,omitempty"`
}

// PlanCommandK0sUpdateStatus is the status of a `K0sUpdate` command for a collection
// of both controllers and workers.
type PlanCommandK0sUpdateStatus struct {
	// Controllers are a collection of status for resolved k0s controller targets.
	Controllers []PlanCommandTargetStatus `json:"controllers,omitempty"`

	// Workers are a collection of status for resolved k0s worker targets.
	Workers []PlanCommandTargetStatus `json:"workers,omitempty"`
}

// PlanCommandAirgapUpdateStatus is the status of a `AirgapUpdate` command for
// k0s worker nodes.
type PlanCommandAirgapUpdateStatus struct {
	// Workers are a collection of status for resolved k0s worker targets.
	Workers []PlanCommandTargetStatus `json:"workers,omitempty"`
}

// PlanCommandTargetStateType is the state of a PlanCommandTarget
type PlanCommandTargetStateType PlanStateType

// PlanCommandTargetStatus is the status of a resolved node (controller/worker).
type PlanCommandTargetStatus struct {
	// Name the name of the target signal node.
	Name string `json:"name"`

	// State is the current state of the target signal nodes operation.
	State PlanCommandTargetStateType `json:"state"`

	// LastUpdatedTimestamp is a timestamp of the last time the status has changed.
	LastUpdatedTimestamp metav1.Time `json:"lastUpdatedTimestamp"`
}
