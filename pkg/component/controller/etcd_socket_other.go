//go:build !linux

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"os"
)

// ensureUnixSocketMode ensures that the unix socket at path has the given
// file mode, reporting whether the mode has been changed. It refuses to
// operate on anything that's not a unix socket and doesn't follow symbolic
// links, albeit with a small TOCTOU window between the check and the chmod
// (production k0s controllers run on Linux, which has a race-free
// implementation).
func ensureUnixSocketMode(path string, mode os.FileMode) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode().Type() != os.ModeSocket {
		return false, fmt.Errorf("not a unix socket: %s", path)
	}
	if info.Mode().Perm() == mode.Perm() {
		return false, nil
	}
	if err := os.Chmod(path, mode.Perm()); err != nil {
		return false, err
	}
	return true, nil
}
