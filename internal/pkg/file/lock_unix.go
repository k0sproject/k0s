//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"os"

	"golang.org/x/sys/unix"
)

func lock(f *os.File, exclusive, wait bool) (bool, error) {
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	if !wait {
		how |= unix.LOCK_NB
	}

	if err := unix.Flock(int(f.Fd()), how); err == nil {
		return true, nil
	} else if err == unix.EWOULDBLOCK {
		return false, nil
	} else {
		return false, os.NewSyscallError("flock", err)
	}
}
