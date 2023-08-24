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
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/stretchr/testify/require"
)

func TestArgsFeatureGates(t *testing.T) {
	featureGates := FeatureGates{
		{
			Name:    "ServiceInternalTrafficPolicy",
			Enabled: true,
		},
	}
	t.Run("if_no_feature_gates_add_new_one", func(t *testing.T) {
		args := stringmap.StringMap{}
		args = featureGates.BuildArgs(args, KubernetesComponents[0])
		require.Equal(t, "ServiceInternalTrafficPolicy=true", args["feature-gates"])
	})
	t.Run("if_args_has_some_argument_preserve_it", func(t *testing.T) {
		args := stringmap.StringMap{
			"some-argument": "value",
		}
		args = featureGates.BuildArgs(args, KubernetesComponents[0])
		require.Equal(t, "ServiceInternalTrafficPolicy=true", args["feature-gates"])
		require.Equal(t, "value", args["some-argument"])
	})
	t.Run("merge_new_feature_gate_with_the_current", func(t *testing.T) {
		args := stringmap.StringMap{
			"feature-gates": "Magic=true",
		}
		args = featureGates.BuildArgs(args, KubernetesComponents[0])
		require.Equal(t, "Magic=true,ServiceInternalTrafficPolicy=true", args["feature-gates"])
	})

	t.Run("enabled_for_component", func(t *testing.T) {
		enabledFeature := FeatureGate{
			Name:       "some_feature_gate",
			Enabled:    true,
			Components: []string{"component_a"},
		}

		explicitlyDisabledFeature := FeatureGate{
			Name:       "another_feature_gate",
			Enabled:    false,
			Components: []string{"component_a"},
		}

		featureGateWithDefaultComponents := FeatureGate{
			Name:    "all_k8s_related_components",
			Enabled: true,
		}
		v, found := enabledFeature.EnabledFor("component_a")
		require.True(t, v, "Must be true for explicitly set component")
		require.True(t, found, "Must be true for explicitly set component")

		v, found = enabledFeature.EnabledFor("component_b")
		require.False(t, v, "Must be false for non set component")
		require.False(t, found, "Must be false for non set component")

		v, found = explicitlyDisabledFeature.EnabledFor("component_a")
		require.False(t, v, "Disabled feature gate must always be false")
		require.True(t, found, "Disabled feature gate must always be found")

		v, found = explicitlyDisabledFeature.EnabledFor("component_b")
		require.False(t, v, "Not enabled feature gate must always be false")
		require.False(t, found, "Not enabled feature gate must always be not found")

		for _, k8sComponent := range KubernetesComponents {
			v, found := featureGateWithDefaultComponents.EnabledFor(k8sComponent)
			require.True(t, v, "Must be true for all k8s related components")
			require.True(t, found, "Must be true for all k8s related components")
		}

		v, found = featureGateWithDefaultComponents.EnabledFor("something")
		require.False(t, v)
		require.False(t, found)
	})

	t.Run("to_string", func(t *testing.T) {
		enabledFeature := FeatureGate{
			Name:       "some_feature_gate",
			Enabled:    true,
			Components: []string{"component_a"},
		}

		explicitlyDisabledFeature := FeatureGate{
			Name:       "another_feature_gate",
			Enabled:    false,
			Components: []string{"component_a"},
		}

		require.Equal(t, "some_feature_gate=true", enabledFeature.String("component_a"))
		require.Equal(t, "", enabledFeature.String("component_b"))
		require.Equal(t, "another_feature_gate=false", explicitlyDisabledFeature.String("component_a"))

	})

	t.Run("feature_gate_validation", func(t *testing.T) {
		validFeature := FeatureGate{
			Name: "some_feature_gate",
		}

		invalidFeatureGate := FeatureGate{}

		require.NoError(t, validFeature.Validate())
		require.Error(t, invalidFeatureGate.Validate())
	})

	t.Run("feature_gates_as_slice_of_strings", func(t *testing.T) {
		featureGates := FeatureGates{
			{
				Name:    "FeatureGate1",
				Enabled: true,
			},
			{
				Name:    "FeatureGate2",
				Enabled: false,
			},
			{
				Name:    "FeatureGate3",
				Enabled: true,
			},
		}
		require.Equal(t, []string{"FeatureGate1=true", "FeatureGate2=false", "FeatureGate3=true"}, featureGates.AsSliceOfStrings("kubelet"))
	})
	t.Run("feature_gates_as_map", func(t *testing.T) {
		featureGates := FeatureGates{
			{
				Name:    "FeatureGate1",
				Enabled: true,
			},
			{
				Name:    "FeatureGate2",
				Enabled: false,
			},
			{
				Name:    "FeatureGate3",
				Enabled: true,
			},
		}
		require.Equal(t, map[string]bool{"FeatureGate1": true, "FeatureGate2": false, "FeatureGate3": true}, featureGates.AsMap("kubelet"))
	})
}
