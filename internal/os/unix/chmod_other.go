//go:build unix && !linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix

import (
	"os"
)

// Delegates to [os.File.Chmod].
func chmod(f *os.File, mode os.FileMode) error {
	return f.Chmod(mode)
}
