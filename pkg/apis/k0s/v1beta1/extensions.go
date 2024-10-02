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
	"errors"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"
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
	Name      string        `json:"name"`
	ChartName string        `json:"chartname"`
	Version   string        `json:"version"`
	Values    string        `json:"values"`
	TargetNS  string        `json:"namespace"`
	Timeout   time.Duration `json:"timeout"`
	// ForceUpgrade when set to false, disables the use of the "--force" flag when upgrading the the chart (default: true).
	// +optional
	ForceUpgrade *bool `json:"forceUpgrade,omitempty"`
	Order        int   `json:"order"`
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
