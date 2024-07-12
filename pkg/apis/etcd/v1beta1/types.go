/*
Copyright 2024 k0s authors

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

package v1beta1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EtcdMember describes the nodes etcd membership status
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Peer Address",type=string,JSONPath=`.status.peerAddress`
// +kubebuilder:printcolumn:name="Member ID",type=string,JSONPath=`.status.memberID`
// +kubebuilder:printcolumn:name="Joined",type=string,JSONPath=`.status.conditions[?(@.type=="Joined")].status`
// +kubebuilder:printcolumn:name="Reconcile Status",type=string,JSONPath=`.status.reconcileStatus`
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update,updateStatus,patch
// +genclient:nonNamespaced
type EtcdMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status Status `json:"status,omitempty"`

	Spec EtcdMemberSpec `json:"spec,omitempty"`
}

// +kubebuilder:validation:Enum=Joined;Left
type JoinStatus string

const (
	JoinStatusJoined JoinStatus = "Joined"
	JoinStatusLeft   JoinStatus = "Left"
)

// EtcdMemberSpec defines the desired state of EtcdMember
type EtcdMemberSpec struct {
	// Leave is a flag to indicate that the member should be removed from the cluster
	Leave bool `json:"leave,omitempty"`
}

type Status struct {
	// PeerAddress is the address of the etcd peer
	PeerAddress string `json:"peerAddress"`
	// MemberID is the unique identifier of the etcd member.
	// The hex form ID is stored as string
	// +kubebuilder:validation:Pattern="^[a-fA-F0-9]+$"
	MemberID string `json:"memberID"`
	// ReconcileStatus is the last status of the reconcile process
	ReconcileStatus string `json:"reconcileStatus,omitempty"`
	Message         string `json:"message,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []JoinCondition `json:"conditions,omitempty"`
}

type ConditionType string

const (
	ConditionTypeJoined ConditionType = "Joined"
)

// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition.
// "ConditionFalse" means a resource is not in the condition. "ConditionUnknown" means kubernetes
// can't decide if a resource is in the condition or not.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type JoinCondition struct {
	// +kubebuilder:validation:Enum=Joined
	Type   ConditionType   `json:"type"`
	Status ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

func (s *Status) GetCondition(conditionType ConditionType) *JoinCondition {
	for _, c := range s.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

func (s *Status) SetCondition(t ConditionType, status ConditionStatus, msg string, time time.Time) {
	var joinCondition JoinCondition
	for i, j := range s.Conditions {
		if j.Type == t {
			jc := &s.Conditions[i]
			// We found the matching type, update it
			// Also if the status changes, update the timestamp
			if jc.Status != status {
				jc.LastTransitionTime = metav1.NewTime(time)
			}
			jc.Status = status
			jc.Message = msg

			return
		}
	}

	joinCondition = JoinCondition{
		Type:               t,
		Status:             status,
		Message:            msg,
		LastTransitionTime: metav1.Now(),
	}

	// We did not find the right typed condition
	if len(s.Conditions) == 0 {
		s.Conditions = []JoinCondition{joinCondition}
	} else {
		s.Conditions = append(s.Conditions, joinCondition)
	}
}

// EtcdMemberList contains a list of EtcdMembers
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type EtcdMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdMember `json:"items"`
}
