// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net/netip"
)

// Return p with an IPv4-mapped IPv6 address converted to IPv4, if the prefix
// can be represented as an IPv4 prefix. Otherwise, it returns p unchanged.
func UnmapPrefix(p netip.Prefix) netip.Prefix {
	a := p.Addr()
	u := a.Unmap()
	if bits := p.Bits(); u != a && bits >= 96 {
		return netip.PrefixFrom(u, bits-96)
	}
	return p
}
