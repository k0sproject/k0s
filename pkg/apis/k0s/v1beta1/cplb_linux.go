// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func getDefaultNIC() (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return "", fmt.Errorf("failed to list routes: %w", err)
	}

	for _, route := range routes {
		if route.Dst.IP == nil ||
			route.Dst.IP.Equal(net.IPv4zero) ||
			route.Dst.IP.Equal(net.IPv6zero) {
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				return "", err
			}
			return link.Attrs().Name, nil
		}
	}

	return "", errors.New("default route not found")
}
