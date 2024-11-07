//go:build unix

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// An open handle to some path on the file system.
type Path interface {
	io.Closer
	syscall.Conn
	Name() string               // Delegates to [os.File.Name].
	Stat() (os.FileInfo, error) // Delegates to [os.File.Stat].

	// Converts this pointer to an [*os.File] without any additional checks.
	//
	// Note that both [os.File.ReadDir] and [os.File.Readdir] will NOT work,
	// even if this path is pointing to a directory.
	UnwrapFile() *os.File

	// Converts this pointer to a [*Dir] without any additional checks.
	//
	// Note that [Dir.Readdirnames] will NOT work if this path is not pointing
	// to a directory.
	UnwrapDir() *Dir
}

// Opens a [Path] referring to the given path.
//
// This function can be used to open a path without knowing if it's a directory
// or a file, then use [Path.Stat] to figure out if [Path.UnwrapFile] or
// [Path.UnwrapDir] is appropriate.
//
// Note that, in contrast to [os.Open] and [os.OpenFile], the returned
// descriptor is not put into non-blocking mode automatically. Callers may
// decide if they want this by setting the [syscall.O_NONBLOCK] flag.
func OpenPath(path string, flags int, perm os.FileMode) (Path, error) {
	// Use the raw syscall instead of os.OpenFile here, as the latter tries to
	// put the fds into non-blocking mode.
	flags, mode, err := sysOpenFlags(flags, perm)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	fd, err := syscall.Open(path, flags, mode)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	return (*pathFD)(os.NewFile(uintptr(fd), path)), nil
}

// A file descriptor pointing to a directory (a.k.a. dirfd). It uses the
// syscalls that accept a dirfd, i.e. openat, fstatat ...
//
// Using a Dir, as opposed to using a path (or path prefix) for all
// operations, offers some unique features: Operations are more consistent. A
// Dir ensures that all operations are relative to the same directory
// instance. If the directory is renamed or moved, the Dir remains valid and
// operations continue to work as expected, which is not the case when using
// paths. Using a Dir can also be more secure. If a directory path is given as
// a string and used repeatedly, there's a risk that the path could be
// maliciously altered (e.g., through symbolic link attacks). Using a Dir
// ensures that operations use the original directory, mitigating this type of
// attack.
//
// Dir implements [fs.StatFS] and can be used as such. However, it can't be
// meaningfully used with [fs.WalkDir]: That function is implemented in terms of
// file system path manipulation, which contradicts the nature of a Dir. For
// this reason, the [fs.File] instances returned by [Dir.Open] won't implement
// [fs.ReadDirFile].
type Dir os.File

// The interface that [Dir] is about to implement.
var _ fs.StatFS = (*Dir)(nil)

// Opens a [Dir] referring to the given path.
//
// Note that this is not a chroot: The *at syscalls will only use Dir to
// resolve relative paths, and will happily follow symlinks and cross mount
// points.
func OpenDir(path string, flags int) (*Dir, error) {
	// Use the raw syscall instead of os.OpenFile here, as the latter tries to
	// put the fds into non-blocking mode.
	fd, err := syscall.Open(path, flags|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	return (*Dir)(os.NewFile(uintptr(fd), path)), nil
}

// Delegates to [os.File.Close].
func (d *Dir) Close() error { return (*os.File)(d).Close() }

// Delegates to [os.File.SyscallConn].
func (d *Dir) SyscallConn() (syscall.RawConn, error) { return (*os.File)(d).SyscallConn() }

// Delegates to [os.File.Name].
func (d *Dir) Name() string { return (*os.File)(d).Name() }

// Delegates to [io.File.Stat].
func (d *Dir) StatSelf() (os.FileInfo, error) { return (*os.File)(d).Stat() }

// Opens the path with the given name.
// The path is opened relative to the receiver, using the openat syscall.
//
// Note that, in contrast to [os.Open] and [os.OpenFile], the returned
// descriptor is not put into non-blocking mode automatically. Callers may
// decide if they want this by setting the [unix.O_NONBLOCK] flag.
//
// https://www.man7.org/linux/man-pages/man2/open.2.html
func (d *Dir) OpenAt(name string, flags int, mode os.FileMode) (Path, error) {
	f, err := d.openAt(name, flags, mode)
	return (*pathFD)(f), err
}

func (d *Dir) openAt(name string, flags int, mode os.FileMode) (*os.File, error) {
	var opened int
	err := syscallControl(d, func(fd uintptr) error {
		flags, mode, err := sysOpenFlags(flags, mode)
		if err != nil {
			return &os.PathError{Op: "openat", Path: name, Err: err}
		}

		opened, err = unix.Openat(int(fd), name, flags, mode)
		if err != nil {
			return &os.PathError{Op: "openat", Path: name, Err: err}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(opened), name), nil
}

// Open implements [fs.FS].
//
// Note that files and directories opened via this method won't implement
// [fs.ReadDirFile] and hence cannot be traversed via the io/fs package.
// However, the returned files will implement [Path], and can then be unwrapped
// to a [*Dir], if appropriate.
func (d *Dir) Open(name string) (fs.File, error) {
	f, err := d.openAt(name, syscall.O_NONBLOCK, 0)
	return (*pathFD)(f), err
}

// Implements [Path] and [fs.File], but hides the [os.File.ReadDir] method.
type pathFD os.File

func (f *pathFD) Close() error                          { return (*os.File)(f).Close() }
func (f *pathFD) Name() string                          { return (*os.File)(f).Name() }
func (f *pathFD) SyscallConn() (syscall.RawConn, error) { return (*os.File)(f).SyscallConn() }
func (f *pathFD) Read(b []byte) (int, error)            { return (*os.File)(f).Read(b) }
func (f *pathFD) Stat() (fs.FileInfo, error)            { return (*os.File)(f).Stat() }
func (f *pathFD) UnwrapFile() *os.File                  { return (*os.File)(f) }
func (f *pathFD) UnwrapDir() *Dir                       { return (*Dir)(f) }

// Delegates to [os.File.Readdirnames].
//
// This is the preferred way of listing directory contents. Traversing can be
// done via [Dir.OpenAt], followed by [Path.Stat] and [Path.UnwrapDir], if
// appropriate. Both [os.File.ReadDir] and [os.File.Readdir] won't make sense
// for Dirs, as they are path based, and not file descriptor based.
func (d *Dir) Readdirnames(n int) ([]string, error) {
	return (*os.File)(d).Readdirnames(n)
}

// Stats the path with the given name.
// The name is interpreted relative to the receiver, using the fstatat syscall.
//
// https://www.man7.org/linux/man-pages/man2/stat.2.html
func (d *Dir) StatAt(name string, flags int) (*FileInfo, error) {
	info := FileInfo{Path: name}
	if err := syscallControl(d, func(fd uintptr) error {
		if err := unix.Fstatat(int(fd), name, (*unix.Stat_t)(&info.Stat), flags); err != nil {
			return &os.PathError{Op: "fstatat", Path: name, Err: err}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &info, nil
}

// Stat implements [fs.StatFS].
func (d *Dir) Stat(name string) (fs.FileInfo, error) {
	fileInfo, err := d.StatAt(name, 0)
	return fileInfo, err
}

type Stat unix.Stat_t

func (s *Stat) ToFileMode() os.FileMode { return toFileMode(s.Mode) }
func (s *Stat) IsDir() bool             { return s.Mode&unix.S_IFMT == unix.S_IFDIR }
func (s *Stat) ModTime() time.Time      { return time.Unix(s.Mtim.Unix()) }
func (s *Stat) Sys() any                { return (*unix.Stat_t)(s) }

type FileInfo struct {
	Path string
	Stat
}

func (i *FileInfo) Name() string      { return filepath.Base(i.Path) }
func (i *FileInfo) Size() int64       { return i.Stat.Size }
func (i *FileInfo) Mode() os.FileMode { return i.ToFileMode() }

func toFileMode[T ~uint16 | ~uint32](unixMode T) os.FileMode {
	fileMode := os.FileMode(unixMode) & os.ModePerm

	// https://www.man7.org/linux/man-pages/man2/fstatat.2.html#EXAMPLES

	switch unixMode & unix.S_IFMT {
	case unix.S_IFREG: // regular file
		// nothing to do
	case unix.S_IFDIR: // directory
		fileMode |= os.ModeDir
	case unix.S_IFIFO: // FIFO/pipe
		fileMode |= os.ModeNamedPipe
	case unix.S_IFLNK: // symlink
		fileMode |= os.ModeSymlink
	case unix.S_IFSOCK: // socket
		fileMode |= os.ModeSocket
	case unix.S_IFCHR: // character device
		fileMode |= os.ModeCharDevice
		fallthrough
	case unix.S_IFBLK: // block device
		fileMode |= os.ModeDevice
	default: // unknown?
		fileMode |= os.ModeIrregular
	}

	if unixMode&unix.S_ISGID != 0 {
		fileMode |= os.ModeSetgid
	}
	if unixMode&unix.S_ISUID != 0 {
		fileMode |= os.ModeSetuid
	}
	if unixMode&unix.S_ISVTX != 0 {
		fileMode |= os.ModeSticky
	}

	return fileMode
}

func sysOpenFlags(flags int, mode os.FileMode) (int, uint32, error) {
	const mask = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	if mode != (mode & mask) {
		return 0, 0, errors.New("invalid mode bits")
	}
	if mode != 0 && flags|os.O_CREATE == 0 {
		return 0, 0, errors.New("mode may only be used when creating")
	}

	return flags | syscall.O_CLOEXEC, toSysMode(mode), nil
}

func toSysMode(mode os.FileMode) uint32 {
	sysMode := uint32(mode & os.ModePerm)
	if mode&os.ModeSetuid != 0 {
		sysMode |= syscall.S_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		sysMode |= syscall.S_ISGID
	}
	if mode&os.ModeSticky != 0 {
		sysMode |= syscall.S_ISVTX
	}
	return sysMode
}

func syscallControl[C syscall.Conn](conn C, f func(fd uintptr) error) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	outerErr := rawConn.Control(func(fd uintptr) { err = f(fd) })
	if outerErr != nil {
		return outerErr
	}
	return err
}
