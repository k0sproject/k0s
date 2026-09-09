// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package net_test

import (
	"net/netip"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/stretchr/testify/assert"
)

func TestUnmapPrefix(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   netip.Prefix
		want netip.Prefix
	}{
		{
			"IPv4",
			netip.MustParsePrefix("10.96.0.0/12"),
			netip.MustParsePrefix("10.96.0.0/12"),
		},
		{
			"IPv6",
			netip.MustParsePrefix("2001:db8::/32"),
			netip.MustParsePrefix("2001:db8::/32"),
		},
		{
			"IPv4-mapped IPv6",
			netip.MustParsePrefix("::ffff:10.96.0.0/108"),
			netip.MustParsePrefix("10.96.0.0/12"),
		},
		{
			"IPv4-mapped IPv6 host prefix",
			netip.MustParsePrefix("::ffff:10.96.0.1/128"),
			netip.MustParsePrefix("10.96.0.1/32"),
		},
		{
			"IPv4-mapped IPv6 /96",
			netip.MustParsePrefix("::ffff:10.96.0.1/96"),
			netip.MustParsePrefix("10.96.0.1/0"),
		},
		{
			"IPv4-mapped IPv6 shorter than /96",
			netip.MustParsePrefix("::ffff:10.96.0.1/95"),
			netip.MustParsePrefix("::ffff:10.96.0.1/95"),
		},
		{
			"unmasked IPv4-mapped IPv6",
			netip.MustParsePrefix("::ffff:10.96.1.2/108"),
			netip.MustParsePrefix("10.96.1.2/12"),
		},
		{
			"invalid prefix",
			netip.Prefix{},
			netip.Prefix{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, net.UnmapPrefix(tt.in))
		})
	}
}
