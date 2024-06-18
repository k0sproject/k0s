/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package iface

import (
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

// AllAddresses returns a list of all network addresses on a node
func AllAddresses() ([]string, error) {
	addresses, err := CollectAllIPs()
	if err != nil {
		return nil, err
	}
	strings := make([]string, len(addresses))
	for i, addr := range addresses {
		strings[i] = addr.String()
	}
	return strings, nil
}

// CollectAllIPs returns a list of all network addresses on a node
func CollectAllIPs() (addresses []net.IP, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	for _, a := range addrs {
		// check the address type and skip if loopback
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				addresses = append(addresses, ipnet.IP)
			}
		}
	}

	return addresses, nil
}

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
		addresses, err := i.Addrs()
		if err != nil {
			logrus.Warnf("failed to get addresses for interface %s: %s", i.Name, err.Error())
			continue
		}
		for _, a := range addresses {
			// check the address type and skip if loopback
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
				if ipnet.IP.To16() != nil && ipv6addr == "" {
					ipv6addr = ipnet.IP.String()
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
