// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package featuregate

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type Feature string

type FeatureGates struct {
	featureMap map[Feature]FeatureGate
}

type stage string

const (
	Alpha stage = "ALPHA"
	Beta  stage = "BETA"
	GA    stage = "GA"

	Deprecated stage = "DEPRECATED"
)

type FeatureGate struct {
	// Enabled determines if the feature is enabled.
	// When declared, it should set to its default value, which may be
	// overwritten based on CLI flags.
	Enabled bool
	// PreRelease indicates the stage of the feature
	Stage stage
}

// isEnabled checks if a feature is enabled.
func (fg *FeatureGates) isEnabled(feature Feature) bool {
	if f, ok := fg.featureMap[feature]; ok {
		return f.Enabled
	}
	return false
}

// ConfigureFeatureGates parses the provided flag string and configures the
// feature gates accordingly. This function is not thread-safe and is only
// expected to be called once during the initialization phase.
func (fg *FeatureGates) configureFeatureGates(flag string) error {
	featureMap, err := parseFeatureGateFlags(flag)
	if err != nil {
		return err
	}

	fg.setDefaults()

	// Apply the parsed feature gates
	for feature, enabled := range featureMap {
		gate, exists := fg.featureMap[Feature(feature)]
		if !exists {
			return fmt.Errorf("unknown feature gate %q", feature)
		}
		if gate.Stage == GA && !enabled {
			// GA features cannot be disabled, so revert the change before returning an error
			return fmt.Errorf("feature gate %q is GA and cannot be disabled", feature)
		}
		gate.Enabled = enabled
		fg.featureMap[Feature(feature)] = gate
	}

	return nil
}

func (fg *FeatureGates) setDefaults() {
	for key, featureGate := range fg.featureMap {
		featureGate.Enabled = featureGate.Stage != Alpha
		fg.featureMap[key] = featureGate
	}
}

// parseFeatureGateFlags parses a comma-separated string of key=value pairs
// and returns a map of feature names to their enabled state.
func parseFeatureGateFlags(flag string) (map[string]bool, error) {
	if flag == "" {
		return nil, nil
	}

	featureMap := make(map[string]bool)

	for pair := range strings.SplitSeq(flag, ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("invalid feature gate format %q, expected key=value", pair)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if key == "" {
			return nil, fmt.Errorf("empty feature gate key in %q", pair)
		}

		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid feature gate value %q for key %q, expected true or false", value, key)
		}
		featureMap[key] = enabled
	}

	return featureMap, nil
}

// IsEnabled checks if a feature is enabled in the defaultFeatureGates singleton
// This function expects that defaultFeatureGates has been initialized and otherwise
// will panic.
func IsEnabled(feature Feature) bool {
	return defaultFeatureGates.isEnabled(feature)
}

// Set initializes the default feature gates with the provided val string.
// This function must be called only once during the initialization phase
// by spf13/pflag while parsing command line flags.
func (fg *FeatureGates) Set(val string) error {
	if defaultFeatureGates != nil {
		return errors.New("feature gates already initialized")
	}
	fg.featureMap = defaultFeatureMap()

	err := fg.configureFeatureGates(val)
	if err != nil {
		return fmt.Errorf("failed to set default feature gates: %w", err)
	}
	defaultFeatureGates = fg
	return nil
}

func (fg *FeatureGates) String() string {
	if fg.featureMap == nil {
		return ""
	}
	features := slices.Collect(maps.Keys(fg.featureMap))
	slices.Sort(features)

	var b strings.Builder
	first := true
	for _, feature := range features {
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString(string(feature))
		b.WriteString("=")
		b.WriteString(strconv.FormatBool(fg.featureMap[feature].Enabled))
	}
	return b.String()
}

func (fg *FeatureGates) Type() string {
	return "mapStringBool"
}

// FlushDefaultFeatureGates clears the default feature gates singleton.
// This is useful for tests to ensure a clean state before each test run
func FlushDefaultFeatureGates(t *testing.T) {
	if t == nil {
		panic("testing.T is nil, cannot flush default feature gates")
	}
	defaultFeatureGates = nil
}
