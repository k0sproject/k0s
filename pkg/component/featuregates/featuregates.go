// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

// Package featuregates converts the feature gates from the k0s cluster
// configuration into the two forms consumed by Kubernetes components: the
// --feature-gates CLI flag value ("Name=true,Name=false") and the
// map[string]bool used in component configuration files. Both match what
// k8s.io/component-base/featuregate parses via Set and SetFromMap.
package featuregates

import (
	"iter"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
)

// Based on args, returns a new argument map in which the feature-gates flag
// includes all the given gates that apply to the given component, formatted in
// the way the component's --feature-gates CLI flag expects. Feature gates are
// appended to any feature-gates value already present in args, so user-provided
// extra arguments are preserved.
func ToArgs(args stringmap.StringMap, gates k0sv1beta1.FeatureGates, component k0sv1beta1.FeatureComponent) stringmap.StringMap {
	var newValue strings.Builder

	oldValueLen, _ := newValue.WriteString(args["feature-gates"])
	for name, enabled := range forComponent(gates, component) {
		if newValue.Len() > 0 {
			newValue.WriteByte(',')
		}
		newValue.WriteString(name)
		newValue.WriteByte('=')
		newValue.WriteString(strconv.FormatBool(enabled))
	}

	args = maps.Clone(args)
	if newValue.Len() == oldValueLen {
		return args
	}

	if args == nil {
		return stringmap.StringMap{"feature-gates": newValue.String()}
	}

	args["feature-gates"] = newValue.String()
	return args
}

// Returns the feature gates that apply to the given component as a map, in the
// form used by the FeatureGates field of component configuration files such as
// KubeletConfiguration and KubeProxyConfiguration.
func ToMap(gates k0sv1beta1.FeatureGates, component k0sv1beta1.FeatureComponent) map[string]bool {
	return maps.Collect(forComponent(gates, component))
}

// Yields the names and enabled states of the feature gates that apply to the
// given component.
func forComponent(gates k0sv1beta1.FeatureGates, component k0sv1beta1.FeatureComponent) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		for _, feature := range gates {
			if isComponentSelected(feature.Components, component) {
				if !yield(feature.Name, feature.Enabled) {
					return
				}
			}
		}
	}
}

func isComponentSelected(components []k0sv1beta1.FeatureComponent, component k0sv1beta1.FeatureComponent) bool {
	return len(components) == 0 || slices.Contains(components, component)
}
