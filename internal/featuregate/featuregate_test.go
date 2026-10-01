// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package featuregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureFeatureGates(t *testing.T) {
	createTestFG := func() *FeatureGates {
		return &FeatureGates{
			map[Feature]FeatureGate{
				Feature("Alpha1"): {Stage: Alpha},
				Feature("Alpha2"): {Stage: Alpha},
				Feature("Beta1"):  {Stage: Beta},
				Feature("Beta2"):  {Stage: Beta},
				Feature("GA"):     {Stage: GA},
			},
		}
	}

	t.Run("default configuration", func(t *testing.T) {
		testFG := createTestFG()
		err := testFG.configureFeatureGates("")
		assert.NoError(t, err, "Expected no error when configuring with empty string")
		assert.False(t, testFG.isEnabled(Feature("Alpha1")), "Expected Alpha1 to be disabled")
		assert.False(t, testFG.isEnabled(Feature("Alpha2")), "Expected Alpha2 to be disabled")
		assert.True(t, testFG.isEnabled(Feature("Beta1")), "Expected Beta1 to be enabled")
		assert.True(t, testFG.isEnabled(Feature("Beta2")), "Expected Beta2 to be enabled")
		assert.True(t, testFG.isEnabled(Feature("GA")), "Expected GA to be enabled")
	})

	t.Run("valid configuration", func(t *testing.T) {
		testFG := createTestFG()
		err := testFG.configureFeatureGates("Alpha1=true,Alpha2=false,Beta1=true,Beta2=false,GA=true")
		assert.NoError(t, err, "Expected no error when configuring with valid features")
		assert.True(t, testFG.isEnabled(Feature("Alpha1")), "Expected Alpha1 to be enabled")
		assert.False(t, testFG.isEnabled(Feature("Alpha2")), "Expected Alpha2 to be disabled")
		assert.True(t, testFG.isEnabled(Feature("Beta1")), "Expected Beta1 to be enabled")
		assert.False(t, testFG.isEnabled(Feature("Beta2")), "Expected Beta2 to be disabled")
		assert.True(t, testFG.isEnabled(Feature("GA")), "Expected GA to be enabled")
	})

	t.Run("invalid feature fails", func(t *testing.T) {
		testFG := createTestFG()
		err := testFG.configureFeatureGates("InvalidFeature=true")
		assert.Error(t, err, "Expected error when configuring with invalid feature")
	})

	t.Run("GA cannot be disabled", func(t *testing.T) {
		testFG := createTestFG()
		err := testFG.configureFeatureGates("GA=false")
		assert.Error(t, err, "Expected error when trying to disable GA feature")
		assert.True(t, testFG.isEnabled(Feature("GA")), "Expected GA to still be enabled after failed disable attempt")
	})
}

func TestSetDefaults(t *testing.T) {
	t.Run("set defaults for different stages", func(t *testing.T) {
		testFG := &FeatureGates{
			map[Feature]FeatureGate{
				Feature("Alpha"): {Enabled: true, Stage: Alpha},
				Feature("Beta"):  {Stage: Beta},
				Feature("GA"):    {Stage: GA},
			},
		}
		testFG.setDefaults()
		assert.False(t, testFG.isEnabled(Feature("Alpha")))
		assert.True(t, testFG.isEnabled(Feature("Beta")))
		assert.True(t, testFG.isEnabled(Feature("GA")))
	})
}

func TestParseFeatureGateFlags(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]bool
		expectError bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "multiple features",
			input:    "Feature1=true,Feature2=false, Feature3 = true ",
			expected: map[string]bool{"Feature1": true, "Feature2": false, "Feature3": true},
		},
		{
			name:     "case insensitive values",
			input:    "Feature1=TRUE,Feature2=False,Feature3=True",
			expected: map[string]bool{"Feature1": true, "Feature2": false, "Feature3": true},
		},
		{
			name:        "missing value",
			input:       "Feature1",
			expectError: true,
		},
		{
			name:        "empty key",
			input:       " = true",
			expectError: true,
		},
		{
			name:        "invalid value",
			input:       "Feature1=maybe",
			expectError: true,
		},
		{
			name:        "multiple equals signs",
			input:       "Feature1=true=false",
			expectError: true,
		},
		{
			name:        "only spaces",
			input:       "   ",
			expectError: true,
		},
		{
			name:        "empty entries forbidden",
			input:       "Feature1=true,,Feature2=false,",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFeatureGateFlags(tt.input)

			if tt.expectError {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}

			if tt.expected == nil {
				assert.Nil(t, result, "Expected result to be nil for empty input")
				return
			}
			assert.Len(t, result, len(tt.expected), "Expected feature map length does not match")

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "Expected value for key %q does not match", key)
			}
		})
	}
}

func TestNewFeatureGates(t *testing.T) {
	t.Run("Singleton", func(t *testing.T) {
		t.Cleanup(func() { defaultFeatureGates = nil })
		fg := FeatureGates{}

		require.Nil(t, defaultFeatureGates)
		err := fg.Set("")
		require.NoError(t, err, "expected first call to succeed")

		err = fg.Set("")
		require.Error(t, err, "expected error on second call")
		assert.EqualError(t, err, "feature gates already initialized")
	})
}

func TestIsEnabled(t *testing.T) {
	t.Run("PanicsIfNotInitialized", func(t *testing.T) {
		t.Cleanup(func() { defaultFeatureGates = nil })

		require.Nil(t, defaultFeatureGates)
		assert.Panics(t, func() {
			_ = IsEnabled(IPv6SingleStack)
		}, "expected panic when defaultFeatureGates is nil")
	})
}

func TestString(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		fg := &FeatureGates{}
		assert.Empty(t, fg.String(), "Expected empty feature gates to return an empty string")
		fg.featureMap = map[Feature]FeatureGate{}
		assert.Empty(t, fg.String(), "Expected empty feature gates to return an empty string")
	})
	t.Run("one element", func(t *testing.T) {
		fg := &FeatureGates{}
		assert.Empty(t, fg.String(), "Expected empty feature gates to return an empty string")

		fg.featureMap = map[Feature]FeatureGate{
			Feature("Feature1"): {Enabled: false},
		}
		assert.Equal(t, "Feature1=false", fg.String(), "Expected string representation to match")
	})

	t.Run("multiple elements", func(t *testing.T) {
		fg := &FeatureGates{}
		assert.Empty(t, fg.String(), "Expected empty feature gates to return an empty string")

		fg.featureMap = map[Feature]FeatureGate{
			Feature("Feature1"): {Enabled: false},
			Feature("Feature2"): {Enabled: true},
			Feature("Feature3"): {Enabled: true},
		}
		assert.Equal(t, "Feature1=false,Feature2=true,Feature3=true", fg.String(), "Expected string representation to match")
	})
}
