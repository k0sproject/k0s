// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/k0sproject/k0s/static"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAllFeatureComponents_UniqueSorted(t *testing.T) {
	for i := range allFeatureComponents {
		if slices.Contains(allFeatureComponents[:i], allFeatureComponents[i]) {
			assert.Failf(t, "Duplicate component value", "%q at index %d", allFeatureComponents[i], i)
		}
	}

	expected := slices.Clone(allFeatureComponents[:])
	slices.Sort(expected)
	assert.Equalf(t, expected, allFeatureComponents[:], "AllFeatureComponents is not sorted")
}

func TestAllFeatureComponents_MatchesCRD(t *testing.T) {
	crdBytes, err := fs.ReadFile(static.CRDs, "k0s/k0s.k0sproject.io_clusterconfigs.yaml")
	require.NoError(t, err)

	var crd map[string]any
	require.NoError(t, yaml.Unmarshal(crdBytes, &crd))

	versions, _, err := unstructured.NestedSlice(crd, "spec", "versions")
	require.NoError(t, err)
	require.Len(t, versions, 1)
	version, ok := versions[0].(map[string]any)
	require.True(t, ok, "Wrong type: %T", versions[0])
	require.Equal(t, "v1beta1", version["name"])

	enum, found, err := unstructured.NestedStringSlice(version,
		"schema", "openAPIV3Schema",
		"properties", "spec",
		"properties", "featureGates", "items",
		"properties", "components", "items",
		"enum",
	)
	require.NoError(t, err)
	require.True(t, found, "No enum for .spec.featureGates[].components[]")

	components := make([]FeatureComponent, len(enum))
	for idx, value := range enum {
		components[idx] = FeatureComponent(value)
	}

	assert.Equal(t, allFeatureComponents[:], components,
		"CRD enum is out of sync with allFeatureComponents; update the kubebuilder marker and run make codegen")
}

func TestFeatureGate_Validate(t *testing.T) {
	for _, test := range []struct {
		name string
		gate FeatureGate
		errs []*field.Error
	}{
		{
			name: "named",
			gate: FeatureGate{Name: "Feature"},
		},
		{
			name: "all components",
			gate: FeatureGate{Name: "Feature", Components: slices.Clone(allFeatureComponents[:])},
		},
		{
			name: "missing name",
			gate: FeatureGate{},
			errs: []*field.Error{field.Required(field.NewPath("name"), "")},
		},
		{
			name: "duplicate component",
			gate: FeatureGate{Name: "Feature", Components: []FeatureComponent{
				FeatureComponentKubelet, FeatureComponentKubelet,
			}},
			errs: []*field.Error{
				field.Duplicate(field.NewPath("components").Index(1), FeatureComponentKubelet),
			},
		},
		{
			name: "unsupported component",
			gate: FeatureGate{Name: "Feature", Components: []FeatureComponent{"bogus"}},
			errs: []*field.Error{
				field.NotSupported(field.NewPath("components").Index(0), FeatureComponent("bogus"), allFeatureComponents[:]),
			},
		},
		{
			name: "reports every error",
			gate: FeatureGate{Components: []FeatureComponent{"bogus", "bogus"}},
			errs: []*field.Error{
				field.Required(field.NewPath("name"), ""),
				field.NotSupported(field.NewPath("components").Index(0), FeatureComponent("bogus"), allFeatureComponents[:]),
				field.Duplicate(field.NewPath("components").Index(1), FeatureComponent("bogus")),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.errs, slices.Collect(test.gate.Validate(nil)))
		})
	}

	t.Run("stops when the yield function returns false", func(t *testing.T) {
		gate := FeatureGate{Components: []FeatureComponent{"bogus"}}
		var collected []*field.Error
		for err := range gate.Validate(nil) {
			collected = append(collected, err)
			break
		}

		require.Len(t, collected, 1)
		assert.Equal(t, collected[0], field.Required(field.NewPath("name"), ""))
	})
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
	t.Run("valid", func(t *testing.T) {
		underTest := FeatureGates{
			{Name: "First"},
			{Name: "Second", Components: []FeatureComponent{FeatureComponentKubelet}},
		}

		assert.Empty(t, slices.Collect(underTest.Validate(nil)))
	})

	t.Run("reports errors for every gate", func(t *testing.T) {
		underTest := FeatureGates{
			{Name: "Feature"},
			{Name: "Feature"},
			{},
			{Name: "Other", Components: []FeatureComponent{"bogus"}},
		}

		var root *field.Path
		assert.Equal(t, []*field.Error{
			field.Duplicate(root.Index(1).Child("name"), "Feature"),
			field.Required(root.Index(2).Child("name"), ""),
			field.NotSupported(root.Index(3).Child("components").Index(0), FeatureComponent("bogus"), allFeatureComponents[:]),
		}, slices.Collect(underTest.Validate(root)))
	})
}

func TestFeatureGates_Sanitized(t *testing.T) {
	for _, test := range []struct {
		name      string
		gates     FeatureGates
		sanitized FeatureGates // nil means "already sane"
	}{
		{
			name: "nil",
		},
		{
			name:  "empty",
			gates: FeatureGates{},
		},
		{
			name: "sane without components",
			gates: FeatureGates{
				{Name: "Feature", Enabled: true},
			},
		},
		{
			name: "sane with components",
			gates: FeatureGates{
				{Name: "Feature", Components: slices.Clone(allFeatureComponents[:])},
			},
		},
		{
			name: "strips unknown components",
			gates: FeatureGates{
				{Name: "Feature", Enabled: true, Components: []FeatureComponent{
					"bogus", FeatureComponentKubelet, "another-bogus",
				}},
			},
			sanitized: FeatureGates{
				{Name: "Feature", Enabled: true, Components: []FeatureComponent{FeatureComponentKubelet}},
			},
		},
		{
			name: "omits gates whose components are all unknown",
			gates: FeatureGates{
				{Name: "First", Components: []FeatureComponent{"bogus"}},
				{Name: "Second", Enabled: true},
			},
			sanitized: FeatureGates{
				{Name: "Second", Enabled: true},
			},
		},
		{
			name: "may omit all gates",
			gates: FeatureGates{
				{Name: "Feature", Components: []FeatureComponent{"bogus"}},
			},
			sanitized: FeatureGates{},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := test.gates.DeepCopy()

			assert.Equal(t, test.sanitized, test.gates.Sanitized())
			assert.Equal(t, original, test.gates, "Feature gates were modified in-place")
		})
	}
}
