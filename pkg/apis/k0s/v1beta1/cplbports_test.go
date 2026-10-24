// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"math"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiPort               = 6443
	konnectivityAgentPort = 8132
	k0sAPIPort            = 9443
)

// allPorts load balances the konnectivity agent and k0s API ports as well.
func allPorts() *LoadBalancedPorts {
	return &LoadBalancedPorts{Konnectivity: new(true), K0sAPI: new(true)}
}

func TestKeepalivedSpec_BalancedPorts(t *testing.T) {
	for _, test := range []struct {
		name string
		spec KeepalivedSpec
		want []BalancedPort
	}{
		{"defaults", KeepalivedSpec{}, []BalancedPort{{apiPort, defaultUserSpaceProxyPort}}},
		{
			"konnectivity", KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{Konnectivity: new(true)}},
			[]BalancedPort{{apiPort, defaultUserSpaceProxyPort}, {konnectivityAgentPort, defaultUserSpaceProxyPort + 1}},
		},
		{
			"all ports", KeepalivedSpec{LoadBalancedPorts: allPorts()},
			[]BalancedPort{{apiPort, defaultUserSpaceProxyPort}, {konnectivityAgentPort, defaultUserSpaceProxyPort + 1}, {k0sAPIPort, defaultUserSpaceProxyPort + 2}},
		},
		{
			"k0s API only", KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{API: new(false), K0sAPI: new(true)}},
			[]BalancedPort{{k0sAPIPort, defaultUserSpaceProxyPort + 2}},
		},
		{"no ports", KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{API: new(false)}}, nil},
		{
			"custom proxy port", KeepalivedSpec{UserSpaceProxyPort: 7000, LoadBalancedPorts: &LoadBalancedPorts{K0sAPI: new(true)}},
			[]BalancedPort{{apiPort, 7000}, {k0sAPIPort, 7002}},
		},
		{"disabled", KeepalivedSpec{DisableLoadBalancer: true, LoadBalancedPorts: &LoadBalancedPorts{Konnectivity: new(true)}}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.spec.BalancedPorts(apiPort, konnectivityAgentPort, k0sAPIPort),
				"exactly the enabled ports should be load balanced, on the right proxy ports")
		})
	}

	spec := &KeepalivedSpec{}
	require.Empty(t, spec.Validate(field.NewPath("keepalived")), "the defaults should be valid")
	assert.Equal(t, defaultUserSpaceProxyPort, spec.UserSpaceProxyPort, "the proxy should listen on the default port for the API server port")
}

func TestLoadBalancedPorts_YAML(t *testing.T) {
	cfg, err := ConfigFromBytes([]byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
spec:
  network:
    controlPlaneLoadBalancing:
      enabled: true
      type: Keepalived
      keepalived:
        vrrpInstances:
        - virtualIPs: ["10.0.0.100/24"]
          authPass: secret
        loadBalancedPorts:
          konnectivity: true
`))
	require.NoError(t, err, "the configuration should parse")
	assert.Equal(t, []BalancedPort{{apiPort, defaultUserSpaceProxyPort}, {konnectivityAgentPort, defaultUserSpaceProxyPort + 1}},
		cfg.Spec.Network.ControlPlaneLoadBalancing.Keepalived.BalancedPorts(apiPort, konnectivityAgentPort, k0sAPIPort),
		"the API server port should stay load balanced, and the konnectivity agent port too, as configured")
}

func TestLoadBalancedPorts_Validate(t *testing.T) {
	virtualServers := VirtualServers{{IPAddress: "10.0.0.100"}}
	for _, test := range []struct {
		spec KeepalivedSpec
		name string
		err  string
	}{
		{KeepalivedSpec{}, "defaults", ""},
		{KeepalivedSpec{LoadBalancedPorts: allPorts()}, "all ports", ""},
		{KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{API: new(false)}}, "no ports", ""},
		{KeepalivedSpec{VirtualServers: virtualServers}, "virtual servers", ""},
		{
			KeepalivedSpec{VirtualServers: virtualServers, LoadBalancedPorts: allPorts()},
			"all ports with virtual servers", "",
		},
		{
			KeepalivedSpec{VirtualServers: virtualServers, LoadBalancedPorts: &LoadBalancedPorts{API: new(false)}},
			"no API server port with virtual servers", "",
		},
		{
			KeepalivedSpec{UserSpaceProxyPort: math.MaxUint16 - 1, LoadBalancedPorts: &LoadBalancedPorts{K0sAPI: new(true)}},
			"no room for the extra proxy ports", "needs the ports 65534 to 65536",
		},
		{
			KeepalivedSpec{UserSpaceProxyPort: math.MaxUint16 - 1, VirtualServers: virtualServers, LoadBalancedPorts: &LoadBalancedPorts{K0sAPI: new(true)}},
			"no extra proxy ports with virtual servers", "",
		},
		{KeepalivedSpec{UserSpaceProxyPort: math.MaxUint16}, "API server port only at the top", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			errs := test.spec.validateLoadBalancedPorts(field.NewPath("keepalived"))
			if test.err == "" {
				assert.Empty(t, errs, "the configuration should be valid")
			} else if assert.Len(t, errs, 1, "the configuration should be rejected once") {
				assert.ErrorContains(t, errs[0], test.err, "the error should explain the problem")
			}
		})
	}
}
