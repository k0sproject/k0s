// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"testing"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
)

func TestEscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It\'s a "nice" \'test\'`, escapeSingleQuotes(`It's a "nice" \'test'`))
}

func TestProxiedPorts(t *testing.T) {
	keepalived := func(ports *k0sAPI.LoadBalancedPorts) *Keepalived {
		return &Keepalived{
			Config:  &k0sAPI.KeepalivedSpec{UserSpaceProxyPort: 6444, LoadBalancedPorts: ports},
			APIPort: 6443, KonnectivityAgentPort: 8132, K0sAPIPort: 9443,
		}
	}
	for _, test := range []struct {
		ports *k0sAPI.LoadBalancedPorts
		name  string
		want  []proxiedPort
	}{
		{nil, "defaults", []proxiedPort{{6443, 6444}}},
		{&k0sAPI.LoadBalancedPorts{Konnectivity: new(true)}, "konnectivity", []proxiedPort{{6443, 6444}, {8132, 6445}}},
		{
			&k0sAPI.LoadBalancedPorts{Konnectivity: new(true), K0sAPI: new(true)}, "all",
			[]proxiedPort{{6443, 6444}, {8132, 6445}, {9443, 6446}},
		},
		{&k0sAPI.LoadBalancedPorts{API: new(false), K0sAPI: new(true)}, "k0s API only", []proxiedPort{{9443, 6446}}},
		{&k0sAPI.LoadBalancedPorts{API: new(false)}, "none", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, keepalived(test.ports).proxiedPorts())
		})
	}
}

func TestBackends(t *testing.T) {
	routes := backends([]string{"10.0.0.1", "fd00::2"}, 8132)
	addrs := make([]string, 0, len(routes))
	for _, route := range routes {
		addrs = append(addrs, route.Addr)
	}
	assert.Equal(t, []string{"10.0.0.1:8132", "[fd00::2]:8132"}, addrs)
	assert.Empty(t, backends(nil, 6443))
}

func TestRedirectArgs(t *testing.T) {
	assert.Equal(t, []string{
		"-t", "nat", "-A", "PREROUTING", "-p", "tcp",
		"-d", "10.0.0.100", "--dport", "8132",
		"-j", "REDIRECT", "--to-port", "6445",
	}, redirectArgs(iptablesCommandAppend, "10.0.0.100", proxiedPort{port: 8132, bindPort: 6445}))
}
