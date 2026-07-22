// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
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

// BuildArgs builds CLI args using the given args and component name.
func (fgs FeatureGates) BuildArgs(args stringmap.StringMap, component string) stringmap.StringMap {
	args = maps.Clone(args)
	componentFeatureGates := []string{}
	for _, feature := range fgs {
		if feature.AppliesTo(component) {
			componentFeatureGates = append(componentFeatureGates, fmt.Sprintf("%s=%t", feature.Name, feature.Enabled))
		}
	}
	if len(componentFeatureGates) == 0 {
		return args
	}

	fg := args["feature-gates"]
	featureGatesString := strings.Join(componentFeatureGates, ",")
	if fg != "" {
		fg = fmt.Sprintf("%s,%s", fg, featureGatesString)
	} else {
		fg = featureGatesString
	}
	if args == nil {
		return stringmap.StringMap{"feature-gates": fg}
	}
	args["feature-gates"] = fg
	return args
}

// AsMap returns feature gates as map[string]bool, used in kubelet
func (fgs FeatureGates) AsMap(component string) map[string]bool {
	componentFeatureGates := map[string]bool{}
	for _, feature := range fgs {
		if feature.AppliesTo(component) {
			componentFeatureGates[feature.Name] = feature.Enabled
		}
	}
	return componentFeatureGates
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
