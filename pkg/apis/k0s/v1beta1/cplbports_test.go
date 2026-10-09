// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancedPorts_Defaults(t *testing.T) {
	// Only the API server port is load balanced by default, as before.
	for _, ports := range []*LoadBalancedPorts{nil, {}} {
		assert.True(t, ports.IsAPIEnabled())
		assert.False(t, ports.IsKonnectivityEnabled())
		assert.False(t, ports.IsK0sAPIEnabled())
	}

	ports := &LoadBalancedPorts{API: new(false), Konnectivity: new(true), K0sAPI: new(true)}
	assert.False(t, ports.IsAPIEnabled())
	assert.True(t, ports.IsKonnectivityEnabled())
	assert.True(t, ports.IsK0sAPIEnabled())
}

func TestKeepalivedSpec_UserSpaceProxyPorts(t *testing.T) {
	spec := &KeepalivedSpec{}
	require.Empty(t, spec.Validate(field.NewPath("keepalived")))
	assert.Equal(t, 6444, spec.UserSpaceProxyPort)
	assert.Equal(t, 6445, spec.UserSpaceProxyKonnectivityPort())
	assert.Equal(t, 6446, spec.UserSpaceProxyK0sAPIPort())
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
	require.NoError(t, err)
	ports := cfg.Spec.Network.ControlPlaneLoadBalancing.Keepalived.LoadBalancedPorts
	assert.True(t, ports.IsAPIEnabled(), "the API server port stays load balanced")
	assert.True(t, ports.IsKonnectivityEnabled())
	assert.False(t, ports.IsK0sAPIEnabled())
}

func TestLoadBalancedPorts_Validate(t *testing.T) {
	virtualServers := VirtualServers{{IPAddress: "10.0.0.100"}}
	for _, test := range []struct {
		spec KeepalivedSpec
		name string
		err  string
	}{
		{KeepalivedSpec{}, "defaults", ""},
		{KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{Konnectivity: new(true), K0sAPI: new(true)}}, "all ports", ""},
		{KeepalivedSpec{LoadBalancedPorts: &LoadBalancedPorts{API: new(false)}}, "no ports", ""},
		{KeepalivedSpec{VirtualServers: virtualServers}, "virtual servers", ""},
		{
			KeepalivedSpec{VirtualServers: virtualServers, LoadBalancedPorts: &LoadBalancedPorts{Konnectivity: new(true)}},
			"konnectivity with virtual servers", "only supported by the userspace proxy",
		},
		{
			KeepalivedSpec{VirtualServers: virtualServers, LoadBalancedPorts: &LoadBalancedPorts{API: new(false)}},
			"no API server port with virtual servers", "only supported by the userspace proxy",
		},
		{
			KeepalivedSpec{UserSpaceProxyPort: 65534, LoadBalancedPorts: &LoadBalancedPorts{K0sAPI: new(true)}},
			"no room for the extra proxy ports", "needs the ports 65534 to 65536",
		},
		{KeepalivedSpec{UserSpaceProxyPort: 65535}, "API server port only at the top", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			errs := test.spec.validateLoadBalancedPorts(field.NewPath("keepalived"))
			if test.err == "" {
				assert.Empty(t, errs)
			} else if assert.Len(t, errs, 1) {
				assert.ErrorContains(t, errs[0], test.err)
			}
		})
	}
}
