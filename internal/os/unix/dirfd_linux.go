/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unix

import (
	"cmp"
	"os"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

// An open Linux-native handle to some path on the file system.
type LinuxPath interface {
	Path

	// Stats this path using the fstatat(path, "", AT_EMPTY_PATH) syscall.
	StatSelf() (*FileInfo, error)
}

var _ LinuxPath = (*PathFD)(nil)

// Stats this path using the fstatat(path, "", AT_EMPTY_PATH) syscall.
func (p *PathFD) StatSelf() (*FileInfo, error) {
	return p.UnwrapDir().StatSelf()
}

var _ LinuxPath = (*DirFD)(nil)

// Stats this path using the fstatat(path, "", AT_EMPTY_PATH) syscall.
func (d *DirFD) StatSelf() (*FileInfo, error) {
	return d.StatAt("", unix.AT_EMPTY_PATH)
}

// Opens the path with the given name.
// The path is opened relative to the receiver, using the openat2 syscall.
//
// Note that, in contrast to [os.Open] and [os.OpenFile], the returned
// descriptor is not put into non-blocking mode automatically. Callers may
// decide if they want this by setting the [syscall.O_NONBLOCK] flag.
//
// Available since Linux 5.6 (April 2020).
//
// https://www.man7.org/linux/man-pages/man2/openat2.2.html
// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/commit/?id=fddb5d430ad9fa91b49b1d34d0202ffe2fa0e179
func (d *DirFD) Open2(name string, how unix.OpenHow) (*PathFD, error) {
	var opened int
	if err := openAt2Support.guard(func() error {
		return syscallControl(d, func(fd uintptr) (err error) {
			how.Flags |= unix.O_CLOEXEC
			opened, err = unix.Openat2(int(fd), name, &how)
			if err == nil {
				return nil
			}
			return &os.PathError{Op: "openat2", Path: name, Err: err}
		})
	}); err != nil {
		return nil, err
	}

	return (*PathFD)(os.NewFile(uintptr(opened), name)), nil
}

// Opens the directory with the given name by using the openat2 syscall.
//
// See [DirFD.Open2].
func (d *DirFD) OpenDir2(name string, how unix.OpenHow) (*DirFD, error) {
	how.Flags |= unix.O_DIRECTORY
	f, err := d.Open2(name, how)
	return f.UnwrapDir(), err
}

var openAt2Support = runtimeSupport{test: func() error {
	// Try to open the current working directory without requiring any
	// permissions (O_PATH). If that fails, assume that openat2 is unusable.
	var cwd int = unix.AT_FDCWD
	fd, err := unix.Openat2(cwd, ".", &unix.OpenHow{Flags: unix.O_PATH | unix.O_CLOEXEC})
	if err != nil {
		return &os.SyscallError{Syscall: "openat2", Err: syscall.ENOSYS}
	}
	_ = unix.Close(fd)
	return nil
}}

type runtimeSupport struct {
	test func() error
	err  atomic.Pointer[error]
}

func (t *runtimeSupport) guard(f func() error) error {
	if err := t.err.Load(); err != nil {
		if *err == nil {
			return f()
		}
		return *err
	}

	err := f()
	if err == nil {
		t.err.Swap(&err)
		return nil
	}

	testErr := t.test()
	if !t.err.CompareAndSwap(nil, &testErr) {
		testErr = *t.err.Load()
	}
	return cmp.Or(testErr, err)
}
