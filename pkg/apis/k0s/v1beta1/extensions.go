/*
Copyright 2020 k0s authors

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
	"encoding/json"
	"errors"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Validateable = (*ClusterExtensions)(nil)

// ClusterExtensions specifies cluster extensions
type ClusterExtensions struct {
	//+kubebuilder:deprecatedversion:warning="storage is deprecated and will be ignored in 1.30. https://docs.k0sproject.io/stable/examples/openebs".
	Storage *StorageExtension `json:"storage"`
	Helm    *HelmExtensions   `json:"helm"`
}

// HelmExtensions specifies settings for cluster helm based extensions
type HelmExtensions struct {
	ConcurrencyLevel int                  `json:"concurrencyLevel"`
	Repositories     RepositoriesSettings `json:"repositories"`
	Charts           ChartsSettings       `json:"charts"`
}

// RepositoriesSettings repository settings
type RepositoriesSettings []Repository

// ChartsSettings charts settings
type ChartsSettings []Chart

// Validate performs validation
func (rs RepositoriesSettings) Validate() []error {
	var errs []error
	for _, r := range rs {
		if err := r.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Validate performs validation
func (cs ChartsSettings) Validate() []error {
	var errs []error
	for _, c := range cs {
		if err := c.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Validate performs validation
func (he HelmExtensions) Validate() []error {
	var errs []error
	if rErrs := he.Repositories.Validate(); rErrs != nil {
		errs = append(errs, rErrs...)
	}
	if cErrs := he.Charts.Validate(); cErrs != nil {
		errs = append(errs, cErrs...)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Chart single helm addon
type Chart struct {
	Name      string `json:"name"`
	ChartName string `json:"chartname"`
	Version   string `json:"version"`
	Values    string `json:"values"`
	TargetNS  string `json:"namespace"`
	// Timeout specifies the timeout for how long to wait for the chart installation to finish.
	// A duration string is a sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	// +kubebuilder:validation:XIntOrString
	Timeout BackwardCompatibleDuration `json:"timeout,omitempty"`
	// ForceUpgrade when set to false, disables the use of the "--force" flag when upgrading the the chart (default: true).
	// +optional
	ForceUpgrade *bool `json:"forceUpgrade,omitempty"`
	Order        int   `json:"order"`
}

// BackwardCompatibleDuration is a metav1.Duration with a different JSON
// unmarshaler. The unmashaler accepts its value as either a string (e.g.
// 10m15s) or as an integer 64. If the value is of type integer then, for
// backward compatibility, it is interpreted as nano seconds.
type BackwardCompatibleDuration metav1.Duration

// MarshalJSON marshals the BackwardCompatibleDuration to JSON.
func (b BackwardCompatibleDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Duration.String())
}

// UnmarshalJSON attempts unmarshals the provided value into a
// BackwardCompatibleDuration. This function attempts to unmarshal it as a
// string first and if that fails it attempts to parse it as an integer.
func (b *BackwardCompatibleDuration) UnmarshalJSON(data []byte) error {
	var duration metav1.Duration
	ustrerr := duration.UnmarshalJSON(data)
	if ustrerr == nil {
		*b = BackwardCompatibleDuration(duration)
		return nil
	}

	var integer int64
	if err := json.Unmarshal(data, &integer); err != nil {
		// we return the error from the first unmarshal attempt.
		return ustrerr
	}
	metadur := metav1.Duration{Duration: time.Duration(integer)}
	*b = BackwardCompatibleDuration(metadur)
	return nil
}

// Validate performs validation
func (c Chart) Validate() error {
	if c.Name == "" {
		return errors.New("chart must have Name field not empty")
	}
	if err := chartutil.ValidateReleaseName(c.Name); err != nil {
		return err
	}
	if c.ChartName == "" {
		return errors.New("chart must have ChartName field not empty")
	}
	if c.TargetNS == "" {
		return errors.New("chart must have TargetNS field not empty")
	}
	return nil
}

// Repository describes single repository entry. Fields map to the CLI flags for the "helm add" command
type Repository struct {
	// The repository name.
	// +kubebuilder:Validation:Required
	Name string `json:"name"`
	// The repository URL.
	// +kubebuilder:Validation:Required
	URL string `json:"url"`
	// Whether to skip TLS certificate checks when connecting to the repository.
	Insecure *bool `json:"insecure,omitempty"`
	// CA bundle file to use when verifying HTTPS-enabled servers.
	CAFile string `json:"caFile,omitempty"`
	// The TLS certificate file to use for HTTPS client authentication.
	CertFile string `json:"certFile,omitempty"`
	// The TLS key file to use for HTTPS client authentication.
	KeyFile string `json:"keyfile,omitempty"`
	// Username for Basic HTTP authentication.
	Username string `json:"username,omitempty"`
	// Password for Basic HTTP authentication.
	Password string `json:"password,omitempty"`
}

func (r *Repository) IsInsecure() bool {
	// This defaults to true when not explicitly set to false.
	// Better have this the other way round in the next API version.
	return r == nil || r.Insecure == nil || *r.Insecure
}

// Validate performs validation
func (r *Repository) Validate() error {
	if r.Name == "" {
		return errors.New("repository must have Name field not empty")
	}
	if r.URL == "" {
		return errors.New("repository must have URL field not empty")
	}
	return nil
}

// Validate stub for Validateable interface
func (e *ClusterExtensions) Validate() []error {
	if e == nil {
		return nil
	}
	var errs []error
	if e.Helm != nil {
		errs = append(errs, e.Helm.Validate()...)
	}
	if e.Storage != nil {
		errs = append(errs, e.Storage.Validate()...)
	}
	return errs
}

func DefaultStorageExtension() *StorageExtension {
	return &StorageExtension{
		Type:                      ExternalStorage,
		CreateDefaultStorageClass: false,
	}
}

// DefaultExtensions default values
func DefaultExtensions() *ClusterExtensions {
	return &ClusterExtensions{
		Storage: DefaultStorageExtension(),
		Helm: &HelmExtensions{
			ConcurrencyLevel: 5,
		},
	}
}
