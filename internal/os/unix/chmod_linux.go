// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix

import (
	"errors"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// Changes the mode of the given file. This works for O_PATH file descriptors,
// too, for which fchmod(2) would fail with EBADF.
//
// It uses fchmodat2(2) with AT_EMPTY_PATH, which is available since Linux 6.6.
// On older kernels, or if fchmodat2 is blocked by some seccomp profile, it
// falls back to changing the mode via the file descriptor's magic link in
// /proc/self/fd.
func chmod(f *os.File, mode os.FileMode) error {
	return syscallControl(f, func(fd uintptr) error {
		err := unix.Fchmodat(int(fd), "", toSysMode(mode), unix.AT_EMPTY_PATH)
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) {
			err = os.Chmod("/proc/self/fd/"+strconv.FormatUint(uint64(fd), 10), mode)
			if pathErr, ok := errors.AsType[*os.PathError](err); ok {
				err = pathErr.Err
			}
		}
		if err != nil {
			return &os.PathError{Op: "chmod", Path: f.Name(), Err: err}
		}
		return nil
	})
}
