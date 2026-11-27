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

func TestStateFromAddrs(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("fd00::100"), Mask: net.CIDRMask(64, 128)},
	}
	for _, test := range []struct {
		name       string
		virtualIPs []string
		want       string
	}{
		{"IPv4", []string{"10.0.0.1/24"}, "MASTER"},
		{"IPv6", []string{"fd00::100/64"}, "MASTER"},
		{"one of several", []string{"10.0.0.2/24", "fd00::100/64"}, "MASTER"},
		{"other mask", []string{"10.0.0.1/32"}, "MASTER"},
		{"not held", []string{"10.0.0.2/24"}, "BACKUP"},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, err := stateFromAddrs(addrs, test.virtualIPs)
			require.NoError(t, err)
			assert.Equal(t, test.want, state)
		})
	}

	_, err := stateFromAddrs(addrs, []string{"10.0.0.1"})
	assert.Error(t, err)
}
