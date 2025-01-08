/*
Copyright 2025 k0s authors

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
	"iter"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func interfaceAddrs(i net.Interface) (iter.Seq[*net.IPNet], error) {
	link, err := netlink.LinkByName(i.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get link by name: %w", err)
	}

	addresses, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("failed to list IP addresses: %w", err)
	}

	return func(yield func(*net.IPNet) bool) {
		for _, a := range addresses {
			// skip secondary addresses. This is to avoid returning VIPs as the public address
			// https://github.com/k0sproject/k0s/issues/4664
			if a.Flags&unix.IFA_F_SECONDARY != 0 {
				continue
			}

			if a.IPNet != nil {
				if !yield(a.IPNet) {
					return
				}
			}
		}
	}, nil
}
