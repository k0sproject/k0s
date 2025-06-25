//go:build !windows

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

func winExecute(args ...string) error {
	panic("Invariant broken: this function should never be called on non-winodws platforms")
}
