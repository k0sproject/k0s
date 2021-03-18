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
package util

import (
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// AllAddresses returns a list of all network addresses on a node
func AllAddresses() ([]string, error) {
	addresses := make([]string, 0, 5)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list network interfaces")
	}

	for _, a := range addrs {
		// check the address type and skip if loopback
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				addresses = append(addresses, ipnet.IP.String())
			}
		}
	}

	logrus.Debugf("found local addresses: %s", addresses)

	return addresses, nil
}

// FirstPublicAddress return the first found non-local address that's not part of pod network
func FirstPublicAddress() (string, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1", errors.Wrap(err, "failed to list network interfaces")
	}
	for _, i := range ifs {
		if i.Name == "vxlan.calico" {
			// Skip calico interface
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
				if ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	logrus.Warn("failed to find any non-local, non podnetwork addresses on host, defaulting public address to 127.0.0.1")
	return "127.0.0.1", nil
}

// MapMerge merges the input from one map with an existing map, so that we can override entries entry in the existing map
func MapMerge(intpuMap map[string]string, existingMap map[string]string) map[string]string {
	for k := range intpuMap {
		existingMap[k] = intpuMap[k]
	}
	return existingMap
}
