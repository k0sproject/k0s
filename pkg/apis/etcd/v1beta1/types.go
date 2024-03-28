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
	"github.com/k0sproject/k0s/pkg/apis/etcd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// GroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: etcd.GroupName, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&EtcdMember{}, &EtcdMemberList{})
}

// EtcdMember dexcribs the nodes etcd membership status
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="PeerAddress",type=string,JSONPath=`.peerAddress`
// +kubebuilder:printcolumn:name="MemberID",type=string,JSONPath=`.memberID`
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update,updateStatus
// +genclient:nonNamespaced
type EtcdMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// PeerAddress is the address of the etcd peer
	PeerAddress string `json:"peerAddress"`
	// MemberID is the unique identifier of the etcd member.
	// The hex form ID is stored as string
	MemberID string `json:"memberID"`

	Status Status `json:"status,omitempty"`
}

type Status struct {
	ReconcileStatus string `json:"reconcileStatus,omitempty"`
	Message         string `json:"message,omitempty"`
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
