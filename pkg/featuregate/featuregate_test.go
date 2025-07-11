/*
Copyright 2025 k0s authors

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

package featuregate

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FeatureGateSuite struct {
	suite.Suite
}

func (s *FeatureGateSuite) TestConfigureFeatureGates() {
	testFG := &FeatureGates{
		map[Feature]FeatureGate{
			Feature("Feature1"): {Prerelease: Alpha},
			Feature("Feature2"): {Prerelease: Alpha},
			Feature("Feature3"): {Prerelease: Beta},
			Feature("Feature4"): {Prerelease: Beta},
		},
	}

	err := testFG.ConfigureFeatureGates("")
	for feature, gate := range testFG.featureMap {
		s.Falsef(gate.Enabled, "%q should be disabled", feature)
	}
	s.NoError(err, "Expected no error when configuring with empty string")

	err = testFG.ConfigureFeatureGates("AllAlpha=true,Feature2=false,AllBeta=true,Feature4=false")
	s.True(testFG.IsEnabled(Feature("Feature1")), "Expected Feature1 to be enabled")
	s.False(testFG.IsEnabled(Feature("Feature2")), "Expected Feature2 to be disabled")
	s.True(testFG.IsEnabled(Feature("Feature3")), "Expected Feature3 to be enabled")
	s.False(testFG.IsEnabled(Feature("Feature4")), "Expected Feature4 to be disabled")
	s.NoError(err, "Expected no error when configuring with valid flags")

	err = testFG.ConfigureFeatureGates("InvalidFeature=true")
	s.Error(err, "Expected error when configuring with invalid feature")
}

func (s *FeatureGateSuite) TestConfigureGlobalFeatureGates() {
	testFG := &FeatureGates{
		map[Feature]FeatureGate{
			Feature("Feature1"): {Enabled: false, Prerelease: Alpha},
			Feature("Feature2"): {Enabled: true, Prerelease: Alpha},
			Feature("Feature3"): {Enabled: false, Prerelease: Beta},
			Feature("Feature4"): {Enabled: true, Prerelease: Beta},
		},
	}
	testFG.configureGlobalFeatureGates(false, false)
	for feature, gate := range testFG.featureMap {
		s.Falsef(gate.Enabled, "%q should be disabled", feature)
	}
	testFG.configureGlobalFeatureGates(true, true)
	for feature, gate := range testFG.featureMap {
		s.Truef(gate.Enabled, "%q should be enabled", feature)
	}
}

func (s *FeatureGateSuite) TestParseFeatureGateFlags() {
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
			input:    "Feature1=true,Feature2=false,Feature3=true",
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
			name:        "missing equals sign",
			input:       "Feature1true",
			expectError: true,
		},
		{
			name:        "empty key",
			input:       "=true",
			expectError: true,
		},
		{
			name:        "invalid value",
			input:       "Feature1=maybe",
			expectError: true,
		},
		{
			name:        "empty key with spaces",
			input:       " = true",
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
			name:        "only commas",
			input:       ",,,",
			expectError: true,
		},
		{
			name:        "empty entries forbidden",
			input:       "Feature1=true,,Feature2=false,",
			expectError: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			result, err := parseFeatureGateFlags(tt.input)

			if tt.expectError {
				s.Error(err, "Expected an error but got none")
			} else {
				s.NoError(err, "Unexpected error")
			}

			if tt.expected == nil {
				s.Nil(result, "Expected result to be nil for empty input")
				return
			}
			s.Len(result, len(tt.expected), "Expected feature map length does not match")

			for key, expectedValue := range tt.expected {
				s.Equal(expectedValue, result[key], "Expected value for key %q does not match", key)
			}
		})
	}
}

func TestFeatureGateSuite(t *testing.T) {
	featureGateSuite := &FeatureGateSuite{}
	suite.Run(t, featureGateSuite)
}
