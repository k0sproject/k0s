/*


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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChartSpec defines the desired state of Chart
type ChartSpec struct {
	Foo string `json:"foo,omitempty"`
}

// ChartStatus defines the observed state of Chart
type ChartStatus struct {
}

// +kubebuilder:object:root=true

// Chart is the Schema for the charts API
type Chart struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChartSpec   `json:"spec,omitempty"`
	Status ChartStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChartList contains a list of Chart
type ChartList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Chart `json:"items"`
}
