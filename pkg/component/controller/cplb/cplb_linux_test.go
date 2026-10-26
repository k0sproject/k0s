// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"net"
	"testing"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
)

func TestEscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It\'s a "nice" \'test\'`, escapeSingleQuotes(`It's a "nice" \'test'`))
}

func TestProxyTarget(t *testing.T) {
	for _, test := range []struct {
		name, address, vip string
		want               []string
	}{
		{"IPv4", "10.0.0.1", "10.0.0.100", []string{"-j", "DNAT", "--to-destination", "10.0.0.1:6444"}},
		{"IPv6", "fd00::1", "fd00::100", []string{"-j", "DNAT", "--to-destination", "[fd00::1]:6444"}},
		{"other family", "10.0.0.1", "fd00::100", []string{"-j", "REDIRECT", "--to-port", "6444"}},
		{"no address", "", "10.0.0.100", []string{"-j", "REDIRECT", "--to-port", "6444"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			k := &Keepalived{Config: &k0sAPI.KeepalivedSpec{UserSpaceProxyPort: 6444}, Address: test.address}
			assert.Equal(t, test.want, k.proxyTarget(net.ParseIP(test.vip)))
		})
	}
}
