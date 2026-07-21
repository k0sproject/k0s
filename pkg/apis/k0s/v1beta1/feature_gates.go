// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
)

var _ Validateable = (*FeatureGates)(nil)

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

type FeatureComponent string

// The different upstream components that deal with feature gates.
const (
	FeatureComponentKubeAPIServer         FeatureComponent = "kube-apiserver"
	FeatureComponentKubeControllerManager FeatureComponent = "kube-controller-manager"
	FeatureComponentKubeProxy             FeatureComponent = "kube-proxy"
	FeatureComponentKubeScheduler         FeatureComponent = "kube-scheduler"
	FeatureComponentKubelet               FeatureComponent = "kubelet"
)

// FeatureGate specifies single feature gate
type FeatureGate struct {
	// Name of the feature gate
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Enabled or disabled
	Enabled bool `json:"enabled"`
	// Components to use feature gate on. Applies to all Kubernetes components
	// if empty.
	// +listType=set
	Components []FeatureComponent `json:"components,omitempty"`
}

// Validate given feature gate
func (fg *FeatureGate) Validate() error {
	if fg.Name == "" {
		return errors.New("feature gate must have name")
	}
	return nil
}
