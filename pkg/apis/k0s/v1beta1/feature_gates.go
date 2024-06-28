/*
Copyright 2021 k0s authors

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
	"fmt"
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

// BuildArgs build cli args using the given args and component name
func (fgs FeatureGates) BuildArgs(args stringmap.StringMap, component string) stringmap.StringMap {
	componentFeatureGates := fgs.AsSliceOfStrings(component)
	fg, componentHasFeatureGates := args["feature-gates"]
	featureGatesString := strings.Join(componentFeatureGates, ",")
	if componentHasFeatureGates {
		fg = fmt.Sprintf("%s,%s", fg, featureGatesString)
	} else {
		fg = featureGatesString
	}
	args["feature-gates"] = fg
	return args
}

// AsMap returns feature gates as map[string]bool, used in kubelet
func (fgs FeatureGates) AsMap(component string) map[string]bool {
	componentFeatureGates := map[string]bool{}
	for _, feature := range fgs {
		value, found := feature.EnabledFor(component)
		if found {
			componentFeatureGates[feature.Name] = value
		}
	}
	return componentFeatureGates
}

// AsSliceOfStrings returns feature gates as slice of strings, used in arguments
func (fgs FeatureGates) AsSliceOfStrings(component string) []string {
	featureGates := []string{}
	for _, feature := range fgs {
		featureGates = append(featureGates, feature.String(component))
	}
	return featureGates
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

// EnabledFor checks if current feature gate is enabled for a given component
func (fg *FeatureGate) EnabledFor(component string) (value bool, found bool) {
	components := fg.Components
	if len(components) == 0 {
		components = KubernetesComponents
	}

	for _, c := range components {
		if c == component {
			found = true
		}
	}
	if found {
		value = fg.Enabled
	}
	return
}

// Validate given feature gate
func (fg *FeatureGate) Validate() error {
	if fg.Name == "" {
		return fmt.Errorf("feature gate must have name")
	}
	return nil
}

// String represents feature gate as a string
func (fg *FeatureGate) String(component string) string {
	value, found := fg.EnabledFor(component)
	if !found {
		return ""
	}
	return fmt.Sprintf("%s=%t", fg.Name, value)
}
