//go:build !windows

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

func newWindows(name string) Service {
	panic("unreachable: Windows service backend unavailable on this platform")
}
