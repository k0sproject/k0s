// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

// FirstPublicAddress return the first found non-local IPv4 address that's not part of pod network
// if any interface does not have any IPv4 address then return the first found non-local IPv6 address
func FirstPublicAddress() (string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1", fmt.Errorf("failed to list network interfaces: %w", err)
	}
	ipv6addr := ""
	for _, i := range ifs {
		switch {
		// Skip calico CNI interface
		case i.Name == "vxlan.calico":
			continue
		// Skip kube-router CNI interface
		case i.Name == "kube-bridge":
			continue
		// Skip k0s CPLB interface
		case i.Name == "dummyvip0":
			continue
		// Skip kube-router pod CNI interfaces
		case strings.HasPrefix(i.Name, "veth"):
			continue
		// Skip calico pod CNI interfaces
		case strings.HasPrefix(i.Name, "cali"):
			continue
		}

		addresses, err := interfaceAddrs(i)
		if err != nil {
			logrus.WithError(err).Warn("Skipping network interface ", i.Name)
			continue
		}
		for a := range addresses {
			// check the address type and skip if loopback
			if !a.IP.IsLoopback() {
				if a.IP.To4() != nil {
					return a.IP.String(), nil
				}
				if a.IP.To16() != nil && ipv6addr == "" {
					ipv6addr = a.IP.String()
				}
			}
		}
	}
	if ipv6addr != "" {
		return ipv6addr, nil
	}

	logrus.Warn("failed to find any non-local, non podnetwork addresses on host, defaulting public address to 127.0.0.1")
	return "127.0.0.1", nil
}
