//go:build linux || windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

type directories struct {
	dataDir        string
	kubeletRootDir string
	runDir         string
}

// Name returns the name of the step
func (d *directories) Name() string {
	return "remove directories step"
}
