//go:build !linux

// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package probes

func newTotalMemoryProber() totalMemoryProber {
	return func() (uint64, error) {
		return 0, probeUnsupported("Total memory detection unsupported on this platform")
	}
}
