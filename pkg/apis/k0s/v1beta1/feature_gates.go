// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"slices"
)

var _ Validateable = (*FeatureGates)(nil)

// KubernetesComponents default components to use feature gates with
var KubernetesComponents = []string{
	"kube-apiserver",
	"kube-controller-manager",
	"kubelet",
	"kube-scheduler",
	"kube-proxy",
}

// FeatureGates collection of feature gate specs
// +listType=map
// +listMapKey=name
type FeatureGates []FeatureGate

// Validate validates all profiles
func (fgs FeatureGates) Validate() []error {
	var errors []error
	for _, p := range fgs {
		if err := p.Validate(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// FeatureGate specifies single feature gate
type FeatureGate struct {
	// Name of the feature gate
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Enabled or disabled
	Enabled bool `json:"enabled"`
	// Components to use feature gate on
	// Default: kube-apiserver, kube-controller-manager, kubelet, kube-scheduler, kube-proxy
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:default={kube-apiserver,kube-controller-manager,kubelet,kube-scheduler,kube-proxy}
	// +listType=set
	Components []string `json:"components,omitempty"`
}

// Checks if this feature gate applies to a given component or not.
func (fg *FeatureGate) AppliesTo(component string) bool {
	components := fg.Components
	if len(components) == 0 {
		components = KubernetesComponents
	}
	return slices.Contains(components, component)
}

// Validate given feature gate
func (fg *FeatureGate) Validate() error {
	if fg.Name == "" {
		return errors.New("feature gate must have name")
	}
	return nil
}
