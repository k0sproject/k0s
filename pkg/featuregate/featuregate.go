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
	"fmt"
	"strings"
)

type Feature string

type FeatureGates struct {
	featureMap map[Feature]FeatureGate
}

type prerelease string

const (
	Alpha = prerelease("ALPHA")
	Beta  = prerelease("BETA")
	GA    = prerelease("")

	Deprecated = prerelease("DEPRECATED")
)

type FeatureGate struct {
	// Enabled determines if the feature is enabled.
	// When declared, it should set to its default value, which may be
	// overwritten based on CLI flags.
	Enabled bool
	// PreRelease indicates the stage of the feature
	Prerelease prerelease
}

// IsEnabled checks if a feature is enabled.
func (fg *FeatureGates) IsEnabled(feature Feature) bool {
	if f, ok := fg.featureMap[feature]; ok {
		return f.Enabled
	}
	return false
}

// ConfigureFeatureGates parses the provided flag string and configures the
// feature gates accordingly. This function is not thread-safe and is only
// expected to be called once during the initialization phase.
func (fg *FeatureGates) ConfigureFeatureGates(flag string) error {
	featureMap, err := parseFeatureGateFlags(flag)
	if err != nil {
		return err
	}

	alphaEnabled := featureMap[string(allAlphaGate)]
	betaEnabled := featureMap[string(allBetaGate)]
	fg.configureGlobalFeatureGates(alphaEnabled, betaEnabled)

	// Apply the parsed feature gates
	for feature, enabled := range featureMap {
		// Skip global gates, they are already handled
		if feature == string(allAlphaGate) || feature == string(allBetaGate) {
			continue
		}
		if gate, exists := fg.featureMap[Feature(feature)]; exists {
			gate.Enabled = enabled
			fg.featureMap[Feature(feature)] = gate
		} else {
			return fmt.Errorf("unknown feature gate %q", feature)
		}
	}

	return nil
}

// parseFeatureGateFlags parses a comma-separated string of key=value pairs
// and returns a map of feature names to their enabled state.
func parseFeatureGateFlags(flag string) (map[string]bool, error) {
	if flag == "" {
		return nil, nil
	}

	featureMap := make(map[string]bool)
	pairs := strings.Split(flag, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, fmt.Errorf("empty feature gate entry in %q", flag)
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid feature gate format %q, expected key=value", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("empty feature gate key in %q", pair)
		}

		switch strings.ToLower(value) {
		case "true":
			featureMap[key] = true
		case "false":
			featureMap[key] = false
		default:
			return nil, fmt.Errorf("invalid feature gate value %q for key %q, expected true or false", value, key)
		}
	}

	return featureMap, nil
}

func (fg *FeatureGates) configureGlobalFeatureGates(alphaEnabled bool, betaEnabled bool) {
	for feature, featureGate := range fg.featureMap {
		switch featureGate.Prerelease {
		case Alpha:
			featureGate.Enabled = alphaEnabled
		case Beta:
			featureGate.Enabled = betaEnabled
		}
		fg.featureMap[feature] = featureGate
	}
}
