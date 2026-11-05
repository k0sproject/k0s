// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"iter"
	"net"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstPublicAddress(t *testing.T) {
	type iface struct {
		name  string
		addrs []string
	}

	for _, test := range []struct {
		name     string
		ifaces   []iface
		expected string
	}{
		{
			name:     "IPv4",
			ifaces:   []iface{{"lo", []string{"127.0.0.1/8", "::1/128"}}, {"eth0", []string{"10.0.0.2/24"}}},
			expected: "10.0.0.2",
		},
		{
			name:     "IPv4 is preferred over IPv6",
			ifaces:   []iface{{"eth0", []string{"2001:db8::2/64"}}, {"eth1", []string{"192.168.1.2/24"}}},
			expected: "192.168.1.2",
		},
		{
			name:     "IPv6 without IPv4",
			ifaces:   []iface{{"eth0", []string{"2001:db8::2/64", "2001:db8::3/64"}}},
			expected: "2001:db8::2",
		},
		{
			name:     "link-local IPv4 is used if it comes first",
			ifaces:   []iface{{"eth0", []string{"169.254.10.20/16"}}, {"eth1", []string{"192.168.1.2/24"}}},
			expected: "169.254.10.20",
		},
		{
			name: "pod network interfaces are skipped",
			ifaces: []iface{
				{"kube-bridge", []string{"10.244.0.1/24"}},
				{"cali123", []string{"10.244.1.1/32"}},
				{"eth0", []string{"10.0.0.2/24"}},
			},
			expected: "10.0.0.2",
		},
		{
			name:     "no addresses",
			ifaces:   []iface{{"lo", []string{"127.0.0.1/8"}}},
			expected: "127.0.0.1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ifs []net.Interface
			addrs := make(map[string][]*net.IPNet)
			for _, i := range test.ifaces {
				ifs = append(ifs, net.Interface{Name: i.name})
				for _, cidr := range i.addrs {
					ip, ipNet, err := net.ParseCIDR(cidr)
					require.NoError(t, err)
					ipNet.IP = ip
					addrs[i.name] = append(addrs[i.name], ipNet)
				}
			}

			interfaceAddrs := func(i net.Interface) (iter.Seq[*net.IPNet], error) {
				return slices.Values(addrs[i.Name]), nil
			}

			assert.Equal(t, test.expected, firstPublicAddress(ifs, interfaceAddrs))
		})
	}
}
