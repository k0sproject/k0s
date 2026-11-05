// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"fmt"
	"iter"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

// FirstPublicAddress returns the first IP address found on the host's network
// interfaces that is suitable as the default address of this node, skipping
// loopback addresses and the interfaces of the pod network.
//
// Global IPv4 addresses are preferred over global IPv6 addresses. Link-local
// addresses (169.254.0.0/16, fe80::/10) are only used when there's nothing
// better: an IPv4 link-local address is returned as a last resort. IPv6
// link-local addresses are never returned, as they're not usable without a
// zone. If no suitable address is found, 127.0.0.1 is returned.
//
// When no global unicast address is found, the fallback address is returned
// together with an error, since Kubernetes accepts neither link-local nor
// loopback addresses as the API server endpoint or the node IP.
func FirstPublicAddress() (string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1", fmt.Errorf("failed to list network interfaces: %w", err)
	}

	return firstPublicAddress(ifs, interfaceAddrs)
}

func firstPublicAddress(ifs []net.Interface, interfaceAddrs func(net.Interface) (iter.Seq[*net.IPNet], error)) (string, error) {
	var ipv6addr, linkLocalIPv4addr string
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
					if !a.IP.IsLinkLocalUnicast() {
						return a.IP.String(), nil
					}
					if linkLocalIPv4addr == "" {
						linkLocalIPv4addr = a.IP.String()
					}
					continue
				}
				if a.IP.To16() != nil && ipv6addr == "" && !a.IP.IsLinkLocalUnicast() {
					ipv6addr = a.IP.String()
				}
			}
		}
	}
	if ipv6addr != "" {
		return ipv6addr, nil
	}

	fallback := "127.0.0.1"
	if linkLocalIPv4addr != "" {
		fallback = linkLocalIPv4addr
	}
	return fallback, fmt.Errorf("no global unicast address found on the host's network interfaces, using %s: Kubernetes requires a global unicast address", fallback)
}
