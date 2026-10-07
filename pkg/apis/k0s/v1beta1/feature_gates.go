// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"iter"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// FeatureGates collection of feature gate specs
// +listType=map
// +listMapKey=name
type FeatureGates []FeatureGate

// Validate validates all profiles
func (fgs FeatureGates) Validate(path *field.Path) iter.Seq[*field.Error] {
	return func(yield func(*field.Error) bool) {
		for idx := range fgs {
			if name := fgs[idx].Name; name != "" {
				for prev := range idx {
					if fgs[prev].Name == fgs[idx].Name {
						if !yield(field.Duplicate(path.Index(idx).Child("name"), fgs[idx].Name)) {
							return
						}
						break
					}
				}
			}

			for err := range fgs[idx].Validate(path.Index(idx)) {
				if !yield(err) {
					return
				}
			}
		}
	}
}

// +kubebuilder:validation:Enum=kube-apiserver;kube-controller-manager;kube-proxy;kube-scheduler;kubelet
type FeatureComponent string

// The different upstream components that deal with feature gates.
const (
	FeatureComponentKubeAPIServer         FeatureComponent = "kube-apiserver"
	FeatureComponentKubeControllerManager FeatureComponent = "kube-controller-manager"
	FeatureComponentKubeProxy             FeatureComponent = "kube-proxy"
	FeatureComponentKubeScheduler         FeatureComponent = "kube-scheduler"
	FeatureComponentKubelet               FeatureComponent = "kubelet"
)

var allFeatureComponents = [...]FeatureComponent{
	FeatureComponentKubeAPIServer,
	FeatureComponentKubeControllerManager,
	FeatureComponentKubeProxy,
	FeatureComponentKubeScheduler,
	FeatureComponentKubelet,
}

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
func (fg *FeatureGate) Validate(path *field.Path) iter.Seq[*field.Error] {
	return func(yield func(*field.Error) bool) {
		if fg == nil {
			return
		}

		if fg.Name == "" {
			if !yield(field.Required(path.Child("name"), "")) {
				return
			}
		}

		for idx, component := range fg.Components {
			if slices.Contains(fg.Components[:idx], component) {
				if !yield(field.Duplicate(path.Child("components").Index(idx), component)) {
					return
				}

				// This is a duplicate, the previous index has been checked
				// already, no need to do it again.
				continue
			}

			if !slices.Contains(allFeatureComponents[:], component) {
				if !yield(field.NotSupported(path.Child("components").Index(idx), component, allFeatureComponents[:])) {
					return
				}
			}
		}
	}
}
