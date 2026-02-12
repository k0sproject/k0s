//go:build linux || windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

type cni struct{}

// Name returns the name of the step
func (c *cni) Name() string {
	return "CNI leftovers cleanup step"
}
