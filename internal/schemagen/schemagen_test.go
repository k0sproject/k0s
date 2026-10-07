// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package schemagen

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

type testCases struct {
	Validation []struct {
		Name  string
		Input string
		Valid bool
	}
	Structural []struct {
		Name    string
		Schema  string
		Valid   []string
		Invalid []string
	}
}

func loadTestCases(t *testing.T) testCases {
	t.Helper()
	data, err := os.ReadFile("testdata/cases.yaml")
	require.NoError(t, err)
	var cases testCases
	require.NoError(t, yaml.Unmarshal(data, &cases))
	return cases
}

func compile(t *testing.T, document any) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("k0s.json", document))
	schema, err := compiler.Compile("k0s.json")
	require.NoError(t, err)
	return schema
}

func TestGenerate(t *testing.T) {
	cases := loadTestCases(t)
	generated, err := Generate("k0s.k0sproject.io/v1beta1", "ClusterConfig")
	require.NoError(t, err)
	var document any
	require.NoError(t, json.Unmarshal(generated, &document))
	schema := compile(t, document)
	for _, test := range cases.Validation {
		t.Run(test.Name, func(t *testing.T) {
			var input any
			require.NoError(t, json.Unmarshal([]byte(test.Input), &input))
			if test.Valid {
				require.NoError(t, schema.Validate(input))
			} else {
				require.Error(t, schema.Validate(input))
			}
		})
	}
}

func TestStructuralSchemaCases(t *testing.T) {
	cases := loadTestCases(t)
	for _, test := range cases.Structural {
		t.Run(test.Name, func(t *testing.T) {
			var source map[string]any
			require.NoError(t, json.Unmarshal([]byte(test.Schema), &source))
			converted, err := structuralSchema(source, false)
			require.NoError(t, err)
			converted["$schema"] = "http://json-schema.org/draft-07/schema#"
			schema := compile(t, converted)
			for _, input := range test.Valid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(input), &value))
				require.NoError(t, schema.Validate(value))
			}
			for _, input := range test.Invalid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(input), &value))
				require.Error(t, schema.Validate(value))
			}
		})
	}
}

func TestList(t *testing.T) {
	identifiers, err := List()
	require.NoError(t, err)
	require.Contains(t, identifiers, Identifier{APIVersion: "k0s.k0sproject.io/v1beta1", Kind: "ClusterConfig"})
}
