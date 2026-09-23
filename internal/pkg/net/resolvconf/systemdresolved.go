// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package resolvconf

import (
	"bufio"
	"net/netip"
	"os"
	"regexp"
)

// Parses the given resolv.conf file and checks if it contains 127.0.0.53 as the
// only nameserver. Then it is assumed to be the systemd-resolved stub.
func IsSystemdResolvedStub(resolvConfPath string) (bool, error) {
	f, err := os.Open(resolvConfPath)
	if err != nil {
		return false, err
	}

	defer f.Close()

	// This is roughly how glibc and musl do it: check for "nameserver" followed
	// by whitespace, then try to parse the next bytes as IP address,
	// disregarding anything after any additional whitespace.
	// https://sourceware.org/git/?p=glibc.git;a=blob;f=resolv/res_init.c;h=cce842fa9311c5bdba629f5e78c19746f75ef18e;hb=refs/tags/glibc-2.37#l396
	// https://git.musl-libc.org/cgit/musl/tree/src/network/resolvconf.c?h=v1.2.3#n62

	nameserverLine := regexp.MustCompile(`^nameserver\s+(\S+)`)
	stubIP := netip.AddrFrom4([4]byte{127, 0, 0, 53})

	lines := bufio.NewScanner(f)
	systemdResolvedIPSeen := false
	for lines.Scan() {
		match := nameserverLine.FindSubmatch(lines.Bytes())
		if len(match) < 1 {
			continue
		}

		if ip, _ := netip.ParseAddr(string(match[1])); systemdResolvedIPSeen || ip != stubIP {
			return false, nil
		}
		systemdResolvedIPSeen = true
	}
	if err := lines.Err(); err != nil {
		return false, err
	}

	return systemdResolvedIPSeen, nil
}
