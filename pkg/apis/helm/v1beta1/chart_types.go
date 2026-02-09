// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// RepositorySpec describes a Helm repository configuration for a Chart.
// Fields map to the CLI flags for the "helm repo add" command.
type RepositorySpec struct {
	// The repository URL.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url,omitempty"`
	// Username for Basic HTTP authentication.
	Username string `json:"username,omitempty"`
	// Password for Basic HTTP authentication.
	Password string `json:"password,omitempty"`
	// CA bundle file to use when verifying HTTPS-enabled servers.
	CAFile string `json:"caFile,omitempty"`
	// The TLS certificate file to use for HTTPS client authentication.
	CertFile string `json:"certFile,omitempty"`
	// The TLS key file to use for HTTPS client authentication.
	KeyFile string `json:"keyFile,omitempty"`
	// Whether to skip TLS certificate checks when connecting to the repository.
	Insecure *bool `json:"insecure,omitempty"`
}

// ToK0sRepository converts RepositorySpec to k0s Repository type for Helm operations.
// Note: The Name field is not included in RepositorySpec as it's not needed for
// embedded repository configurations.
func (r *RepositorySpec) ToK0sRepository(name string) k0sv1beta1.Repository {
	repo := k0sv1beta1.Repository{
		Name:     name,
		URL:      r.URL,
		Username: r.Username,
		Password: r.Password,
		CAFile:   r.CAFile,
		CertFile: r.CertFile,
		KeyFile:  r.KeyFile,
		Insecure: r.Insecure,
	}
	return repo
}

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
	// Repository configuration for the chart. When specified, the chart will use this
	// repository configuration instead of looking up a repository from the cluster-level
	// repository cache. This enables self-contained Chart resources.
	Repository *RepositorySpec `json:"repository,omitempty"`
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
