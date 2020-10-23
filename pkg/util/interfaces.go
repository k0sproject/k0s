/*
Copyright 2020 Mirantis, Inc.

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

// FirstPublicAddress return the first found non-local address
func FirstPublicAddress() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", errors.Wrap(err, "failed to list network interfaces")
	}

	for _, a := range addrs {
		// check the address type and skip if loopback
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", nil
}
