// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// ensureUnixSocketMode ensures that the unix socket at path has the given
// file mode, reporting whether the mode has been changed. It refuses to
// operate on anything that's not a unix socket and never follows symbolic
// links, so that it cannot be tricked into changing the mode of unrelated
// files by the (potentially less privileged) owner of the socket's
// directory. Since Linux doesn't support AT_SYMLINK_NOFOLLOW for
// fchmodat(2), this uses an O_PATH file descriptor via /proc instead.
func ensureUnixSocketMode(path string, mode os.FileMode) (bool, error) {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return false, &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer func() { _ = unix.Close(fd) }()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return false, &os.PathError{Op: "fstat", Path: path, Err: err}
	}

	switch {
	case stat.Mode&unix.S_IFMT != unix.S_IFSOCK:
		return false, fmt.Errorf("not a unix socket: %s", path)
	case os.FileMode(stat.Mode)&os.ModePerm == mode.Perm():
		return false, nil
	}

	if err := os.Chmod("/proc/self/fd/"+strconv.Itoa(fd), mode.Perm()); err != nil {
		return false, &os.PathError{Op: "chmod", Path: path, Err: err}
	}
	return true, nil
}
