// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"maps"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureGate_Validate(t *testing.T) {
	for _, test := range []struct {
		name string
		gate FeatureGate
		err  string
	}{
		{name: "named", gate: FeatureGate{Name: "Feature"}},
		{name: "missing name", gate: FeatureGate{}, err: "feature gate must have name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.gate.Validate()
			if test.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestFeatureGate_EnabledFor(t *testing.T) {
	for _, test := range []struct {
		name           string
		gate           FeatureGate
		component      string
		enabled, found bool
	}{
		{
			name:      "explicit component enabled",
			gate:      FeatureGate{Enabled: true, Components: []string{"component-a"}},
			component: "component-a",
			enabled:   true, found: true,
		},
		{
			name:      "explicit component disabled",
			gate:      FeatureGate{Components: []string{"component-a"}},
			component: "component-a",
			enabled:   false, found: true,
		},
		{
			name:      "enabled gate does not match component",
			gate:      FeatureGate{Enabled: true, Components: []string{"component-a"}},
			component: "component-b",
			enabled:   false, found: false,
		},
		{
			name:      "disabled gate does not match component",
			gate:      FeatureGate{Components: []string{"component-a"}},
			component: "component-b",
			enabled:   false, found: false,
		},
		{
			name:      "enabled for a default component",
			gate:      FeatureGate{Enabled: true},
			component: "kubelet",
			enabled:   true, found: true,
		},
		{
			name:      "disabled for a default component",
			gate:      FeatureGate{},
			component: "kubelet",
			enabled:   false, found: true,
		},
		{
			name:      "not enabled for a non-default component",
			gate:      FeatureGate{Enabled: true},
			component: "other-component",
			enabled:   false, found: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, found := test.gate.EnabledFor(test.component)
			assert.Equal(t, test.enabled, value)
			assert.Equal(t, test.found, found)
		})
	}
}

func TestFeatureGates_FromConfig(t *testing.T) {
	c, err := ConfigFromBytes([]byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  featureGates:
    - name: feature_XXX
      enabled: true
      components: ["x", "y", "z"]
    - name: feature_YYY
      enabled: true
    - name: feature_ZZZ
      enabled: false
`))
	require.NoError(t, err)
	assert.Equal(t, FeatureGates{
		{Name: "feature_XXX", Enabled: true, Components: []string{"x", "y", "z"}},
		{Name: "feature_YYY", Enabled: true},
		{Name: "feature_ZZZ"},
	}, c.Spec.FeatureGates)
}

func TestFeatureGates_BuildArgs(t *testing.T) {
	someGates := FeatureGates{
		{Name: "DefaultComponents", Enabled: true},
		{Name: "APIServerOnly", Components: []string{"kube-apiserver"}},
	}

	for _, test := range []struct {
		name                                                string
		gates                                               FeatureGates
		args                                                stringmap.StringMap
		expectedDefault, expectedAPIServer, expectedUnknown stringmap.StringMap
	}{
		{
			name:  "nil",
			gates: nil,
			args: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedDefault: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedAPIServer: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedUnknown: stringmap.StringMap{
				"unrelated": "value",
			},
		},
		{
			name:  "empty",
			gates: FeatureGates{},
			args: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedDefault: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedAPIServer: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedUnknown: stringmap.StringMap{
				"unrelated": "value",
			},
		},
		{
			name:  "without existing arguments",
			gates: someGates,
			args:  stringmap.StringMap{},
			expectedDefault: stringmap.StringMap{
				"feature-gates": "DefaultComponents=true",
			},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "DefaultComponents=true,APIServerOnly=false",
			},
			expectedUnknown: stringmap.StringMap{},
		},
		{
			name:  "preserves unrelated arguments",
			gates: someGates,
			args:  stringmap.StringMap{"unrelated": "value"},
			expectedDefault: stringmap.StringMap{
				"feature-gates": "DefaultComponents=true",
				"unrelated":     "value",
			},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "DefaultComponents=true,APIServerOnly=false",
				"unrelated":     "value",
			},
			expectedUnknown: stringmap.StringMap{
				"unrelated": "value",
			},
		},
		{
			name:  "appends to existing feature gates",
			gates: someGates,
			args: stringmap.StringMap{
				"feature-gates": "Existing=true",
			},
			expectedDefault: stringmap.StringMap{
				"feature-gates": "Existing=true,DefaultComponents=true",
			},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "Existing=true,DefaultComponents=true,APIServerOnly=false",
			},
			expectedUnknown: stringmap.StringMap{
				"feature-gates": "Existing=true",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, component := range KubernetesComponents {
				expected := test.expectedDefault
				if component == "kube-apiserver" {
					expected = test.expectedAPIServer
				}
				args := maps.Clone(test.args)

				actual := test.gates.BuildArgs(args, component)

				assert.Equalf(t, test.args, args, "Arguments were modified in-place")
				assert.Equalf(t, expected, actual, "For component %s", component)
			}

			args := maps.Clone(test.args)

			actual := test.gates.BuildArgs(args, "some-unknown-component")

			assert.Equalf(t, test.args, args, "Arguments were modified in-place")
			assert.Equalf(t, test.expectedUnknown, actual, "For some unknown component")
		})
	}
}

func TestFeatureGates_Validate(t *testing.T) {
	errs := (FeatureGates{{}, {Name: "Valid"}, {}}).Validate()
	require.Len(t, errs, 2)
	assert.EqualError(t, errs[0], "feature gate must have name")
	assert.EqualError(t, errs[1], "feature gate must have name")
}

func TestFeatureGates_AsMap(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		actual := FeatureGates{}.AsMap("component")
		assert.Empty(t, actual)
	})

	underTest := FeatureGates{
		{Name: "DefaultComponentsEnabled", Enabled: true},
		{Name: "DefaultComponentsDisabled"},
		{Name: "APIEnabled", Enabled: true, Components: []string{"kube-apiserver"}},
		{Name: "SchedulerEnabled", Enabled: true, Components: []string{"kube-scheduler"}},
	}

	for _, test := range []struct {
		component string
		expected  map[string]bool
	}{
		{
			component: "kube-apiserver",
			expected: map[string]bool{
				"DefaultComponentsEnabled":  true,
				"DefaultComponentsDisabled": false,
				"APIEnabled":                true,
			},
		},
		{
			component: "kube-scheduler",
			expected: map[string]bool{
				"DefaultComponentsEnabled":  true,
				"DefaultComponentsDisabled": false,
				"SchedulerEnabled":          true,
			},
		},
		{
			component: "other",
			expected:  map[string]bool{},
		},
	} {
		t.Run(test.component, func(t *testing.T) {
			assert.Equal(t, test.expected, underTest.AsMap(test.component))
		})
	}
}
