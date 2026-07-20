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
		{Name: "AllComponents", Enabled: true},
		{Name: "APIServerOnly", Components: []string{"kube-apiserver"}},
	}

	for _, test := range []struct {
		name                             string
		gates                            k0sv1beta1.FeatureGates
		args                             stringmap.StringMap
		expectedAPIServer, expectedOther stringmap.StringMap
	}{
		{
			name:  "nil",
			gates: nil,
			args: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedAPIServer: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedOther: stringmap.StringMap{
				"unrelated": "value",
			},
		},
		{
			name:  "empty",
			gates: k0sv1beta1.FeatureGates{},
			args: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedAPIServer: stringmap.StringMap{
				"unrelated": "value",
			},
			expectedOther: stringmap.StringMap{
				"unrelated": "value",
			},
		},
		{
			name:  "without existing arguments",
			gates: someGates,
			args:  stringmap.StringMap{},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "AllComponents=true,APIServerOnly=false",
			},
			expectedOther: stringmap.StringMap{
				"feature-gates": "AllComponents=true",
			},
		},
		{
			name:  "preserves unrelated arguments",
			gates: someGates,
			args:  stringmap.StringMap{"unrelated": "value"},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "AllComponents=true,APIServerOnly=false",
				"unrelated":     "value",
			},
			expectedOther: stringmap.StringMap{
				"feature-gates": "AllComponents=true",
				"unrelated":     "value",
			},
		},
		{
			name:  "appends to existing feature gates",
			gates: someGates,
			args: stringmap.StringMap{
				"feature-gates": "Existing=true",
			},
			expectedAPIServer: stringmap.StringMap{
				"feature-gates": "Existing=true,AllComponents=true,APIServerOnly=false",
			},
			expectedOther: stringmap.StringMap{
				"feature-gates": "Existing=true,AllComponents=true",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := maps.Clone(test.args)
			actual := ToArgs(args, test.gates, "kube-apiserver")
			assert.Equal(t, test.args, args, "Arguments were modified in-place")
			assert.Equal(t, test.expectedAPIServer, actual)

			args = maps.Clone(test.args)
			actual = ToArgs(args, test.gates, "some-other-component")
			assert.Equal(t, test.args, args, "Arguments were modified in-place")
			assert.Equal(t, test.expectedOther, actual)
		})
	}
}

func TestToMap(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		underTest := ToMap(k0sv1beta1.FeatureGates{}, "component")
		assert.Empty(t, underTest)
	})

	featureGates := k0sv1beta1.FeatureGates{
		{Name: "AllComponentsEnabled", Enabled: true},
		{Name: "AllComponentsDisabled"},
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
				"AllComponentsEnabled":  true,
				"AllComponentsDisabled": false,
				"APIEnabled":            true,
			},
		},
		{
			component: "kube-scheduler",
			expected: map[string]bool{
				"AllComponentsEnabled":  true,
				"AllComponentsDisabled": false,
				"SchedulerEnabled":      true,
			},
		},
		{
			component: "other",
			expected: map[string]bool{
				"AllComponentsEnabled":  true,
				"AllComponentsDisabled": false,
			},
		},
	} {
		t.Run(test.component, func(t *testing.T) {
			assert.Equal(t, test.expected, ToMap(featureGates, test.component))
		})
	}
}

func TestForComponent_StopsEarly(t *testing.T) {
	gates := k0sv1beta1.FeatureGates{
		{Name: "First", Enabled: true},
		{Name: "Second"},
	}

	collected := map[string]bool{}
	for name, enabled := range forComponent(gates, "kubelet") {
		collected[name] = enabled
		break
	}

	assert.Equal(t, map[string]bool{"First": true}, collected)
}
