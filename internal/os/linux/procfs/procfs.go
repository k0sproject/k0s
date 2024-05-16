//go:build linux

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package procfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// A proc(5) filesystem.
//
// See https://www.kernel.org/doc/html/latest/filesystems/proc.html.
// See https://man7.org/linux/man-pages/man5/proc.5.html.
type ProcFS string

const (
	DefaultMountPoint        = "/proc"
	Default           ProcFS = DefaultMountPoint
)

func At(mountPoint string) ProcFS {
	return ProcFS(mountPoint)
}

func (p ProcFS) String() string {
	return string(p)
}

func (p ProcFS) Verify() error {
	path, err := filepath.Abs(p.String())
	if err != nil {
		return fmt.Errorf("proc(5) filesystem check failed: %w", err)
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		statErr := &fs.PathError{Op: "statfs", Path: path, Err: err}
		if errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("%w: proc(5) filesystem unavailable", errors.ErrUnsupported)
		} else {
			err = errors.New("proc(5) filesystem check failed")
		}
		return fmt.Errorf("%w: %v", err, statErr) //nolint:errorlint // shadow stat err
	}

	if st.Type != unix.PROC_SUPER_MAGIC {
		return fmt.Errorf("%w: not a proc(5) filesystem: %s: type is 0x%x", errors.ErrUnsupported, p, st.Type)
	}
	return nil
}
