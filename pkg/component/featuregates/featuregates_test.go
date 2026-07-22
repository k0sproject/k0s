// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package featuregates

import (
	"maps"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
)

func TestToArgs(t *testing.T) {
	someGates := k0sv1beta1.FeatureGates{
		{Name: "DefaultComponents", Enabled: true},
		{Name: "APIServerOnly", Components: []string{"kube-apiserver"}},
	}

	for _, test := range []struct {
		name                                                string
		gates                                               k0sv1beta1.FeatureGates
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
			gates: k0sv1beta1.FeatureGates{},
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
			for _, component := range defaultComponents {
				expected := test.expectedDefault
				if component == "kube-apiserver" {
					expected = test.expectedAPIServer
				}
				args := maps.Clone(test.args)

				actual := ToArgs(args, test.gates, component)

				assert.Equalf(t, test.args, args, "Arguments were modified in-place")
				assert.Equalf(t, expected, actual, "For component %s", component)
			}

			args := maps.Clone(test.args)

			actual := ToArgs(args, test.gates, "some-unknown-component")

			assert.Equalf(t, test.args, args, "Arguments were modified in-place")
			assert.Equalf(t, test.expectedUnknown, actual, "For some unknown component")
		})
	}
}

func TestToMap(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		underTest := ToMap(k0sv1beta1.FeatureGates{}, "component")
		assert.Empty(t, underTest)
	})

	featureGates := k0sv1beta1.FeatureGates{
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
			assert.Equal(t, test.expected, ToMap(featureGates, test.component))
		})
	}
}
