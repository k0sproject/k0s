// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"testing"

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
		{Name: "feature_XXX", Enabled: true, Components: []FeatureComponent{"x", "y", "z"}},
		{Name: "feature_YYY", Enabled: true},
		{Name: "feature_ZZZ"},
	}, c.Spec.FeatureGates)
}

func TestFeatureGates_Validate(t *testing.T) {
	errs := (FeatureGates{{}, {Name: "Valid"}, {}}).Validate()
	require.Len(t, errs, 2)
	assert.EqualError(t, errs[0], "feature gate must have name")
	assert.EqualError(t, errs[1], "feature gate must have name")
}
