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
	"strconv"
	"syscall"

	"github.com/k0sproject/k0s/internal/os/linux"
	osunix "github.com/k0sproject/k0s/internal/os/unix"

	"golang.org/x/sys/unix"
)

var _ = linux.SendSignal // for godoc links

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

// Delegates to [Default].
// See [ProcFS.OpenPID].
func OpenPID(pid int) (*osunix.Dir, error) {
	return Default.OpenPID(pid)
}

// Returns a [*osunix.Dir] that points to a process-specific subdirectory inside
// the proc(5) filesystem. It therefore refers to a process or thread, and may
// be used in some syscalls that accept pidfds, most notably [linux.SendSignal].
//
// Operations on open /proc/<pid> Dirs corresponding to dead processes never act
// on any new process that the kernel may, through chance, have also assigned
// the same process ID. Instead, operations on these Dirs usually fail with
// [syscall.ESRCH].
//
// The underlying file descriptor of the Dir obtained in this way is not
// pollable and can't be waited on with waitid(2).
//
// https://docs.kernel.org/filesystems/proc.html#process-specific-subdirectories
func (p ProcFS) OpenPID(pid int) (*osunix.Dir, error) {
	path := filepath.Join(p.String(), strconv.Itoa(pid))
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	pidDir, err := osunix.OpenDir(path, 0)
	if err != nil {
		// If there was an error, check if the procfs is actually valid.
		verifyErr := p.Verify()
		if verifyErr != nil {
			err = fmt.Errorf("%w (%v)", verifyErr, err) //nolint:errorlint // shadow open err
		}
		return nil, err
	}

	return pidDir, nil
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
