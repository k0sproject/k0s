// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
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
		{"defaulted sections", `{"spec":{"api":null,"network":{},"storage":null}}`, true},
		{"null spec", `{"spec":null}`, true},
		{"partial image override", `{"spec":{"images":{"coredns":{"version":"custom"}}}}`, true},
		{"metadata", `{"apiVersion":"k0s.k0sproject.io/v1beta1","kind":"ClusterConfig","metadata":{"name":"k0s","labels":{"example.com/team":"ops"}}}`, true},
		{"extra arguments", `{"spec":{"api":{"extraArgs":{"audit-log-path":"/tmp/audit.log"}}}}`, true},
		{"worker values", `{"spec":{"workerProfiles":[{"name":"custom","values":{"featureGates":{"Example":true},"reservedMemory":[{"numaNode":0,"limits":{"memory":"1Gi"}}],"clusterDNS":["10.96.0.10"],"arbitrary":null}}]}}`, true},
		{"helm duration", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":"5m","values":"replicas: 2"}]}}}}`, true},
		{"legacy helm duration", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":60000000000}]}}}}`, true},
		{"wrong helm duration type", `{"spec":{"extensions":{"helm":{"charts":[{"chartname":"example/app","name":"app","namespace":"default","timeout":true}]}}}}`, false},
		{"unknown section", `{"spec":{"networks":{}}}`, false},
		{"unknown field", `{"spec":{"api":{"prot":6443}}}`, false},
		{"wrong port type", `{"spec":{"api":{"port":"6443"}}}`, false},
		{"wrong boolean type", `{"spec":{"telemetry":{"enabled":"true"}}}`, false},
		{"wrong map value type", `{"spec":{"api":{"extraArgs":{"flag":[]}}}}`, false},
		{"wrong list item type", `{"spec":{"api":{"sans":[42]}}}`, false},
		{"wrong worker values type", `{"spec":{"workerProfiles":[{"name":"custom","values":"text"}]}}`, false},
		{"unknown worker field", `{"spec":{"workerProfiles":[{"name":"custom","vaues":{}}]}}`, false},
		{"wrong root type", `[]`, false},
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
		{"missing feature name", `{"spec":{"featureGates":[{}]}}`, false},
		{"empty feature name", `{"spec":{"featureGates":[{"name":""}]}}`, false},
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

	// Exercise config-file exceptions against both the schema and the real
	// loader/validator, so CRD constraints cannot silently break defaulting.
	t.Run("config file defaults", func(t *testing.T) {
		for _, test := range []struct{ name, input string }{
			{"empty config", `{}`},
			{"default API ports", `{"spec":{"api":{"port":0,"k0sApiPort":0}}}`},
			{"null sections", `{"spec":{"api":null,"storage":null,"images":null}}`},
			{"partial image and empty pull policy", `{"spec":{"images":{"coredns":{"version":"custom"},"default_pull_policy":""}}}`},
			{"partial image and null pull policy", `{"spec":{"images":{"coredns":{"image":"example.com/coredns"},"default_pull_policy":null}}}`},
			{"default feature settings", `{"spec":{"featureGates":[{"name":"ExampleFeature","components":[]}]}}`},
			{"default address family and legacy hairpin", `{"spec":{"network":{"primaryAddressFamily":"","kuberouter":{"hairpin":""}}}}`},
			{"Keepalived defaults", `{"spec":{"network":{"controlPlaneLoadBalancing":{"type":"","keepalived":{"userSpaceProxyBindPort":0,"virtualServers":[{"ipAddress":"127.0.0.2","lbAlgo":"","lbKind":"","persistenceTimeoutSeconds":0}],"vrrpInstances":[{"interface":"lo","authPass":"secret","virtualIPs":["192.0.2.1/24"],"virtualRouterID":0,"advertIntervalSeconds":0}]}}}}}`},
			{"proxy defaults", `{"spec":{"network":{"nodeLocalLoadBalancing":{"type":"","envoyProxy":{"apiServerBindPort":0,"image":{"image":"","version":""},"imagePullPolicy":""},"traefik":{"apiServerBindPort":0,"image":{"version":"custom"},"imagePullPolicy":""}}}}}`},
			{"Helm entries", `{"spec":{"extensions":{"helm":{"charts":[{"name":"app","chartname":"example/app","namespace":"default","timeout":"5m"}],"repositories":[{"name":"example","url":"https://example.com"}]}}}}`},
			{"resource patch", `{"spec":{"metricsServer":{"patches":[{"target":{"kind":"Deployment","name":"metrics-server"},"patch":{"type":"MergePatch","content":"{}"}}]}}}`},
		} {
			t.Run(test.name, func(t *testing.T) {
				config, err := v1beta1.ConfigFromBytes([]byte(test.input))
				require.NoError(t, err)
				require.Empty(t, config.Validate())
				var value any
				require.NoError(t, json.Unmarshal([]byte(test.input), &value))
				require.NoError(t, schema.Validate(value))
			})
		}
	})

	t.Run("default configuration", func(t *testing.T) {
		data, err := json.Marshal(v1beta1.DefaultClusterConfig())
		require.NoError(t, err)
		var value any
		require.NoError(t, json.Unmarshal(data, &value))
		require.NoError(t, schema.Validate(value))
	})

	t.Run("descriptions", func(t *testing.T) {
		properties := document.(map[string]any)["properties"].(map[string]any)
		spec := properties["spec"].(map[string]any)["properties"].(map[string]any)
		api := spec["api"].(map[string]any)["properties"].(map[string]any)
		require.Equal(t, "Address on which to connect to the API server.", api["address"].(map[string]any)["description"])
	})
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
