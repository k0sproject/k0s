// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package resolvconf

import (
	"bufio"
	"errors"
	"io"
	"net/netip"
	"regexp"
)

// https://www.freedesktop.org/software/systemd/man/latest/systemd-resolved.service.html#/etc/resolv.conf
const (
	// The resolv.conf variant that points to systemd-resolved's stub resolver
	// on localhost. This is the recommended target for the /etc/resolv.conf
	// symlink.
	SystemdResolvedStubPath = "/run/systemd/resolve/stub-resolv.conf"

	// The resolv.conf variant that lists systemd-resolved's uplink DNS servers
	// directly. Suitable for consumers that can't reach the host's localhost,
	// such as containers.
	SystemdResolvedUplinkPath = "/run/systemd/resolve/resolv.conf"
)

// Indicates that a resolv.conf file doesn't list any nameservers. Resolvers
// usually fall back to sending DNS queries to a nameserver on localhost then.
var ErrNoNameservers = errors.New("no nameservers configured")

// Parses r as a resolv.conf file and checks if it contains 127.0.0.53 as the
// only nameserver. Then it is assumed to be the systemd-resolved stub. Returns
// [ErrNoNameservers] if r doesn't list any nameservers at all.
func IsSystemdResolvedStub(r io.Reader) (bool, error) {
	// This is roughly how glibc and musl do it: check for "nameserver" followed
	// by whitespace, then try to parse the next bytes as IP address,
	// disregarding anything after any additional whitespace.
	// https://sourceware.org/git/?p=glibc.git;a=blob;f=resolv/res_init.c;h=cce842fa9311c5bdba629f5e78c19746f75ef18e;hb=refs/tags/glibc-2.37#l396
	// https://git.musl-libc.org/cgit/musl/tree/src/network/resolvconf.c?h=v1.2.3#n62

	nameserverLine := regexp.MustCompile(`^nameserver\s+(\S+)`)
	stubIP := netip.AddrFrom4([4]byte{127, 0, 0, 53})

	lines := bufio.NewScanner(r)
	var systemdResolvedIPSeen, anyNameserverSeen bool
	for lines.Scan() {
		match := nameserverLine.FindSubmatch(lines.Bytes())
		if len(match) < 1 {
			continue
		}

		anyNameserverSeen = true
		if ip, _ := netip.ParseAddr(string(match[1])); systemdResolvedIPSeen || ip != stubIP {
			return false, nil
		}
		systemdResolvedIPSeen = true
	}
	if err := lines.Err(); err != nil {
		return false, err
	}

	if !anyNameserverSeen {
		return false, ErrNoNameservers
	}

	return systemdResolvedIPSeen, nil
}
