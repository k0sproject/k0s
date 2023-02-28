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
	Storage *StorageExtension `json:"storage"`
	Helm    *HelmExtensions   `json:"helm"`
}

// HelmExtensions specifies settings for cluster helm based extensions
type HelmExtensions struct {
	Repositories RepositoriesSettings `json:"repositories"`
	Charts       ChartsSettings       `json:"charts"`
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
	Name     string `json:"name"`
	URL      string `json:"url"`
	CAFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	Insecure bool   `json:"insecure"`
	KeyFile  string `json:"keyfile"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Validate performs validation
func (r Repository) Validate() error {
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

// DefaultExtensions default values
func DefaultExtensions() *ClusterExtensions {
	return &ClusterExtensions{
		Storage: &StorageExtension{
			Type:                      ExternalStorage,
			CreateDefaultStorageClass: false,
		},
		Helm: &HelmExtensions{},
	}
}
