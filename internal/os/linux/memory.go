//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"os"

	"golang.org/x/sys/unix"
)

// Returns the total usable main memory size in bytes.
//
// See [man 2 sysinfo].
//
// [man 2 sysinfo]: https://www.man7.org/linux/man-pages/man2/sysinfo.2.html
func TotalMemory() (uint64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, os.NewSyscallError("sysinfo", err)
	}

	//nolint:unconvert // explicit cast to support 32-bit systems
	return uint64(info.Totalram) * uint64(info.Unit), nil
}
