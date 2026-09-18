// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

const crdPath = "../../static/_crds/k0s/k0s.k0sproject.io_clusterconfigs.yaml"

func TestConfigSchema(t *testing.T) {
	crd, err := os.ReadFile(crdPath)
	require.NoError(t, err)
	generated, err := generate(crd)
	require.NoError(t, err)

	var document any
	require.NoError(t, json.Unmarshal(generated, &document))
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("k0s.json", document))
	schema, err := compiler.Compile("k0s.json")
	require.NoError(t, err)

	for _, test := range []struct {
		name  string
		input string
		valid bool
	}{
		{"empty config", `{}`, true},
		{"partial config", `{"spec":{"api":{"port":6443}}}`, true},
		{"null sections", `{"spec":{"api":null,"network":{},"storage":null}}`, false},
		{"null spec", `{"spec":null}`, false},
		{"partial image override", `{"spec":{"images":{"coredns":{"version":"custom"}}}}`, false},
		{"metadata", `{"apiVersion":"k0s.k0sproject.io/v1beta1","kind":"ClusterConfig","metadata":{"name":"k0s","labels":{"example.com/team":"ops"}}}`, true},
		{"extra arguments", `{"spec":{"api":{"extraArgs":{"audit-log-path":"/tmp/audit.log"}}}}`, true},
		{"worker values", `{"spec":{"workerProfiles":[{"name":"custom","values":{"featureGates":{"Example":true},"reservedMemory":[{"numaNode":0,"limits":{"memory":"1Gi"}}],"clusterDNS":["10.96.0.10"],"arbitrary":null}}]}}`, true},
		{"helm duration", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":"5m","values":"replicas: 2"}]}}}}`, true},
		{"legacy helm duration", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":60000000000}]}}}}`, true},
		{"wrong helm duration type", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":true}]}}}}`, false},
		{"unknown section", `{"spec":{"networks":{}}}`, true},
		{"unknown field", `{"spec":{"api":{"prot":6443}}}`, true},
		{"wrong port type", `{"spec":{"api":{"port":"6443"}}}`, false},
		{"wrong boolean type", `{"spec":{"telemetry":{"enabled":"true"}}}`, false},
		{"wrong map value type", `{"spec":{"api":{"extraArgs":{"flag":[]}}}}`, false},
		{"wrong list item type", `{"spec":{"api":{"sans":[42]}}}`, false},
		{"wrong worker values type", `{"spec":{"workerProfiles":[{"name":"custom","values":"text"}]}}`, false},
		{"unknown worker field", `{"spec":{"workerProfiles":[{"name":"custom","vaues":{}}]}}`, true},
		{"wrong root type", `[]`, false},
		{"image override", `{"spec":{"images":{"coredns":{"image":"example.com/coredns","version":"v1.0"}}}}`, true},
		{"image digest", `{"spec":{"images":{"coredns":{"image":"example.com/coredns","version":"v1@sha256:0123456789abcdefABCDEF0123456789abcdefABCDEF0123456789abcdefABCDEF01"}}}}`, true},
		{"invalid image version", `{"spec":{"images":{"coredns":{"image":"example.com/coredns","version":"bad tag"}}}}`, false},
		{"invalid image digest", `{"spec":{"images":{"coredns":{"image":"example.com/coredns","version":"v1@sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}}}}`, false},
		{"zero API port", `{"spec":{"api":{"port":0}}}`, false},
		{"empty pull policy", `{"spec":{"images":{"default_pull_policy":""}}}`, false},
		{"feature without enabled", `{"spec":{"featureGates":[{"name":"ExampleFeature"}]}}`, false},
		{"feature with enabled", `{"spec":{"featureGates":[{"name":"ExampleFeature","enabled":false}]}}`, true},
		{"empty feature components", `{"spec":{"featureGates":[{"name":"ExampleFeature","enabled":false,"components":[]}]}}`, false},
		{"negative API port", `{"spec":{"api":{"port":-1}}}`, false},
		{"oversized API port", `{"spec":{"api":{"k0sApiPort":65536}}}`, false},
		{"zero konnectivity port", `{"spec":{"konnectivity":{"agentPort":0}}}`, false},
		{"invalid pull policy", `{"spec":{"images":{"default_pull_policy":"Alwayz"}}}`, false},
		{"invalid Calico mode", `{"spec":{"network":{"calico":{"mode":"invalid"}}}}`, false},
		{"invalid storage type", `{"spec":{"storage":{"type":"invalid"}}}`, false},
		{"empty chart", `{"spec":{"extensions":{"helm":{"charts":[{}]}}}}`, false},
		{"null chart", `{"spec":{"extensions":{"helm":{"charts":[null]}}}}`, false},
		{"empty repository", `{"spec":{"extensions":{"helm":{"repositories":[{}]}}}}`, false},
		{"empty repository name", `{"spec":{"extensions":{"helm":{"repositories":[{"name":"","url":"https://example.com"}]}}}}`, false},
		{"null repository name", `{"spec":{"extensions":{"helm":{"repositories":[{"name":null,"url":"https://example.com"}]}}}}`, false},
		{"empty patch", `{"spec":{"metricsServer":{"patches":[{}]}}}`, false},
		{"null patch target", `{"spec":{"metricsServer":{"patches":[{"target":null,"patch":{"type":"MergePatch","content":"{}"}}]}}}`, false},
		{"invalid patch type", `{"spec":{"metricsServer":{"patches":[{"target":{"kind":"Deployment","name":"metrics-server"},"patch":{"type":"invalid","content":"{}"}}]}}}`, false},
		{"missing feature name", `{"spec":{"featureGates":[{"enabled":false}]}}`, false},
		{"empty feature name", `{"spec":{"featureGates":[{"name":"","enabled":false}]}}`, false},
		{"missing worker name", `{"spec":{"workerProfiles":[{"values":{}}]}}`, false},
		{"missing etcd endpoints", `{"spec":{"storage":{"etcd":{"externalCluster":{}}}}}`, false},
		{"empty etcd endpoints", `{"spec":{"storage":{"etcd":{"externalCluster":{"endpoints":[]}}}}}`, false},
		{"oversized VRRP ID", `{"spec":{"network":{"controlPlaneLoadBalancing":{"keepalived":{"vrrpInstances":[{"authPass":"secret","virtualIPs":["192.0.2.1/24"],"virtualRouterID":256}]}}}}}`, false},
		{"long VRRP password", `{"spec":{"network":{"controlPlaneLoadBalancing":{"keepalived":{"vrrpInstances":[{"authPass":"too-long-password","virtualIPs":["192.0.2.1/24"]}]}}}}}`, false},
		{"zero proxy konnectivity port", `{"spec":{"network":{"nodeLocalLoadBalancing":{"envoyProxy":{"konnectivityServerBindPort":0}}}}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var value any
			require.NoError(t, json.Unmarshal([]byte(test.input), &value))
			err := schema.Validate(value)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}

	t.Run("descriptions", func(t *testing.T) {
		properties := document.(map[string]any)["properties"].(map[string]any)
		spec := properties["spec"].(map[string]any)["properties"].(map[string]any)
		api := spec["api"].(map[string]any)["properties"].(map[string]any)
		require.Equal(t, "Address on which to connect to the API server.", api["address"].(map[string]any)["description"])
	})
}

func TestStructuralSchema(t *testing.T) {
	for _, test := range []struct {
		name    string
		schema  string
		valid   []string
		invalid []string
	}{
		{"optional is not nullable", `{"type":"object","properties":{"value":{"type":"string"}}}`, []string{`{}`, `{"value":"ok"}`, `{"other":1}`}, []string{`{"value":null}`}},
		{"integer or string", `{"x-kubernetes-int-or-string":true}`, []string{`42`, `"42"`}, []string{`true`, `1.5`, `null`}},
		{"int32 bounds", `{"type":"integer","format":"int32"}`, []string{`-2147483648`, `2147483647`}, []string{`-2147483649`, `2147483648`}},
		{"tighter int32 bounds", `{"type":"integer","format":"int32","minimum":1,"maximum":65535}`, []string{`1`, `65535`}, []string{`0`, `65536`}},
		{"wider int32 bounds", `{"type":"integer","format":"int32","minimum":-3000000000,"maximum":3000000000}`, []string{`-2147483648`, `2147483647`}, []string{`-2147483649`, `2147483648`}},
		{"preserved unknown fields", `{"type":"object","x-kubernetes-preserve-unknown-fields":true,"properties":{"known":{"type":"integer"}}}`, []string{`{"known":1,"arbitrary":{"nested":[null,true]}}`}, []string{`{"known":"bad"}`}},
		{"explicit additional properties", `{"type":"object","properties":{"known":{"type":"integer"}},"additionalProperties":true}`, []string{`{"other":true}`}, []string{`{"known":false}`}},
		{"explicitly closed object", `{"type":"object","properties":{"known":{"type":"integer"}},"additionalProperties":false}`, []string{`{"known":1}`}, []string{`{"other":true}`}},
		{"nested arrays and maps", `{"type":"object","additionalProperties":{"type":"array","items":{"type":"object","properties":{"value":{"type":"string","x-kubernetes-int-or-string":true}}}}}`, []string{`{"key":[{"value":1}]}`, `{"key":[{"value":"text"}]}`, `{"key":[{"other":1}]}`}, []string{`{"key":[{"value":true}]}`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var source map[string]any
			require.NoError(t, json.Unmarshal([]byte(test.schema), &source))
			converted, err := structuralSchema(source)
			require.NoError(t, err)
			converted["$schema"] = "http://json-schema.org/draft-07/schema#"
			compiler := jsonschema.NewCompiler()
			require.NoError(t, compiler.AddResource("test.json", converted))
			schema, err := compiler.Compile("test.json")
			require.NoError(t, err)
			for _, input := range test.valid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(input), &value))
				require.NoError(t, schema.Validate(value), "valid input: %s", input)
			}
			for _, input := range test.invalid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(input), &value))
				require.Error(t, schema.Validate(value), "invalid input: %s", input)
			}
			unchanged, err := json.Marshal(source)
			require.NoError(t, err)
			require.JSONEq(t, test.schema, string(unchanged), "conversion must not mutate the CRD")
		})
	}
}

func TestGenerateRejectsUnsupportedFeatures(t *testing.T) {
	for _, test := range []struct {
		schema string
		error  string
	}{
		{`{"type":"string","nullable":true}`, `unsupported schema keyword "nullable"`},
		{`{"type":"number","exclusiveMinimum":true}`, `unsupported schema keyword "exclusiveMinimum"`},
		{`{"allOf":[{"type":"string"}]}`, `unsupported schema keyword "allOf"`},
		{`{"x-kubernetes-validations":[{"rule":"self > 0"}]}`, `unsupported schema keyword "x-kubernetes-validations"`},
		{`{"type":"integer","format":"int64"}`, `unsupported format int64 for type integer`},
		{`{"type":"string","format":"int32"}`, `unsupported format int32 for type string`},
		{`{"properties":{"value":{"nullable":true}}}`, `property "value": unsupported schema keyword "nullable"`},
		{`{"items":{"nullable":true}}`, `items: unsupported schema keyword "nullable"`},
		{`{"additionalProperties":{"nullable":true}}`, `additionalProperties: unsupported schema keyword "nullable"`},
	} {
		t.Run(test.schema, func(t *testing.T) {
			input := `{"spec":{"versions":[{"name":"v1beta1","schema":{"openAPIV3Schema":` + test.schema + `}}]}}`
			_, err := generate([]byte(input))
			require.EqualError(t, err, test.error)
		})
	}
}

func TestGenerateRequiresVersionedSchema(t *testing.T) {
	for _, input := range []string{
		"[invalid yaml",
		"{}",
		`{"spec":{"versions":[{"name":"v1beta1"}]}}`,
		`{"spec":{"versions":[{"name":"v2","schema":{"openAPIV3Schema":{"type":"object"}}}]}}`,
	} {
		_, err := generate([]byte(input))
		require.Error(t, err)
	}
}

func TestCheckedInSchema(t *testing.T) {
	crd, err := os.ReadFile(crdPath)
	require.NoError(t, err)
	generated, err := generate(crd)
	require.NoError(t, err)
	checkedIn, err := os.ReadFile("../../schemas/k0s.json")
	require.NoError(t, err)
	require.Equal(t, string(generated), string(checkedIn), "run make config-schema")
}
