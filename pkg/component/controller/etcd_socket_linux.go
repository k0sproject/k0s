// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"fmt"
	"os"

	osunix "github.com/k0sproject/k0s/internal/os/unix"

	"golang.org/x/sys/unix"
)

// ensureUnixSocketMode ensures that the unix socket at path has the given
// file mode, reporting whether the mode has been changed. It refuses to
// operate on anything that's not a unix socket and never follows symbolic
// links, so that it cannot be tricked into changing the mode of unrelated
// files by the (potentially less privileged) owner of the socket's
// directory. The socket is opened via an O_PATH file descriptor, which is
// then used for both the type check and the mode change.
func ensureUnixSocketMode(path string, mode os.FileMode) (_ bool, err error) {
	p, err := osunix.OpenPath(path, unix.O_PATH|unix.O_NOFOLLOW, 0)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, p.Close()) }()

	info, err := p.Stat()
	switch {
	case err != nil:
		return false, err
	case info.Mode().Type() != os.ModeSocket:
		return false, fmt.Errorf("not a unix socket: %s", path)
	case info.Mode().Perm() == mode.Perm():
		return false, nil
	}

	if err := p.Chmod(mode.Perm()); err != nil {
		return false, err
	}
	return true, nil
}
