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
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChartSpec defines the desired state of Chart
type ChartSpec struct {
	ChartName string `json:"chartName,omitempty"`
	Values    string `json:"values,omitempty"`
	Version   string `json:"version,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// YamlValues returns values as map
func (cs ChartSpec) YamlValues() map[string]interface{} {
	res := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(cs.Values), &res); err != nil {
		logrus.WithField("values", cs.Values).Warn("broken yaml values")
	}
	return res
}

// ChartStatus defines the observed state of Chart
type ChartStatus struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Status      string `json:"status,omitempty"`
	AppVersion  string `json:"appVersion,omitempty"`
	Updated     string `json:"updated,omitempty"`
	Revision    int    `json:"revision,omitempty"`

	LastChartName string `json:"lastChartName,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// Chart is the Schema for the charts API
type Chart struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChartSpec   `json:"spec,omitempty"`
	Status ChartStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ChartList contains a list of Chart
type ChartList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Chart `json:"items"`
}
