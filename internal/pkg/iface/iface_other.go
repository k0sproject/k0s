//go:build !linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package iface

import (
	"fmt"
	"iter"
	"net"
)

func interfaceAddrs(i net.Interface) (iter.Seq[*net.IPNet], error) {
	addresses, err := i.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to list interface addresses: %w", err)
	}

	return func(yield func(*net.IPNet) bool) {
		for _, a := range addresses {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet != nil {
				if !yield(ipnet) {
					return
				}
			}
		}
	}, nil
}
