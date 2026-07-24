//go:build linux || windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import "time"

type directories struct {
	dataDir        string
	kubeletRootDir string
	runDir         string

	// unmountTimeout bounds each blocking unmount. Overridable in tests.
	unmountTimeout time.Duration
}

// Name returns the name of the step
func (d *directories) Name() string {
	return "remove directories step"
}
