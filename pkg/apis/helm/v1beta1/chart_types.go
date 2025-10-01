// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ChartSpec defines the desired state of Chart
type ChartSpec struct {
	ChartName   string `json:"chartName,omitempty"`
	ReleaseName string `json:"releaseName,omitempty"`
	Values      string `json:"values,omitempty"`
	Version     string `json:"version,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	// ForceUpgrade when set to false, disables the use of the "--force" flag when upgrading the chart (default: true).
	ForceUpgrade *bool `json:"forceUpgrade,omitempty"`
	Order        int   `json:"order,omitempty"`
}

// YamlValues returns values as map
func (cs ChartSpec) YamlValues() map[string]any {
	res := map[string]any{}
	if err := yaml.Unmarshal([]byte(cs.Values), &res); err != nil {
		logrus.WithField("values", cs.Values).Warn("broken yaml values")
	}
	// We need to clean the map to have nested maps as map[string]interface{} types
	// otherwise Helm will fail to merge default values and create the release object
	return CleanUpGenericMap(res)
}

// HashValues returns hash of the values
func (cs ChartSpec) HashValues() string {
	h := sha256.New()
	h.Write([]byte(cs.ReleaseName + cs.Values))
	return hex.EncodeToString(h.Sum(nil))
}

// ShouldForceUpgrade returns true if the chart should be force upgraded
func (cs ChartSpec) ShouldForceUpgrade() bool {
	// This defaults to true when not explicitly set to false.
	// Better have this the other way round in the next API version.
	return cs.ForceUpgrade == nil || *cs.ForceUpgrade
}

// ChartStatus defines the observed state of Chart
type ChartStatus struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Status      string `json:"status,omitempty"`
	AppVersion  string `json:"appVersion,omitempty"`
	Version     string `json:"version,omitempty"`
	Updated     string `json:"updated,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Revision    int64  `json:"revision,omitempty"`
	ValuesHash  string `json:"valuesHash,omitempty"`
	Error       string `json:"error,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
// Chart is the Schema for the charts API
type Chart struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// +optional
	Spec ChartSpec `json:"spec"`
	// +optional
	Status ChartStatus `json:"status"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ChartList contains a list of Chart
type ChartList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Chart `json:"items"`
}
