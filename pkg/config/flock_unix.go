//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"

	"golang.org/x/sys/unix"
)

func lockFile(f *os.File, exclusive bool) (bool, error) {
	how := unix.LOCK_NB
	if exclusive {
		how |= unix.LOCK_EX
	} else {
		how |= unix.LOCK_SH
	}

	if err := unix.Flock(int(f.Fd()), how); err == nil {
		return true, nil
	} else if err == unix.EWOULDBLOCK {
		return false, nil
	} else {
		return false, os.NewSyscallError("flock", err)
	}
}
