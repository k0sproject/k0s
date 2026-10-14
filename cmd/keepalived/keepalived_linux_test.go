// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It's a "nice" 'test'`, unescapeSingleQuotes(`It\'s a "nice" \'test'`))
}

func TestHoldsVirtualIP(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("fd00::100"), Mask: net.CIDRMask(64, 128)},
	}
	for _, test := range []struct {
		name       string
		virtualIPs []string
		want       bool
	}{
		{"held IPv4", []string{"10.0.0.1/24"}, true},
		{"held IPv6", []string{"fd00::100/64"}, true},
		{"one of several held", []string{"10.0.0.2/24", "fd00::100/64"}, true},
		{"not held", []string{"10.0.0.2/24"}, false},
		{"none", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			held, err := holdsVirtualIP(test.virtualIPs, addrs)
			require.NoError(t, err)
			assert.Equal(t, test.want, held, "the state should follow the virtual IPs on the interface")
		})
	}

	_, err := holdsVirtualIP([]string{"10.0.0.1"}, addrs)
	assert.Error(t, err, "a virtual IP without a prefix length should be rejected")
}
