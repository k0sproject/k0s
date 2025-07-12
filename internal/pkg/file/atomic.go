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

package file

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// The internal options for atomic file writes.
type atomicOpts struct {
	target      string
	permissions fs.FileMode
	uid, gid    int
	mtime       time.Time
}

func (o *atomicOpts) wantsChmod() bool {
	return o.permissions.IsRegular()
}

type AtomicOpener struct{ atomicOpts }

// Prepares to open a new [Atomic] for the file at the given target path.
func AtomicWithTarget(target string) *AtomicOpener {
	return &AtomicOpener{atomicOpts{
		target:      target,
		permissions: fs.ModeIrregular, // use this as an "unset" marker, see wantsChmod()
		uid:         -1,
		gid:         -1,
	}}
}

// The desired permissions for the target.
// Will rely on the umask if not called.
func (o *AtomicOpener) WithPermissions(perm os.FileMode) *AtomicOpener {
	o.permissions = perm.Perm()
	return o
}

// The desired modification time for the target file.
func (o *AtomicOpener) WithModificationTime(mtime time.Time) *AtomicOpener {
	o.mtime = mtime
	return o
}

// The desired owner UID for the target file.
// Will be owned by the current user if not called.
// Will have no effect on Windows.
func (o *AtomicOpener) WithOwner(uid int) *AtomicOpener {
	o.uid = max(-1, uid)
	return o
}

// The desired group ID for the target file.
// Will be owned by the current user's group if not called.
// Will have no effect on Windows.
func (o *AtomicOpener) WithGroup(gid int) *AtomicOpener {
	o.gid = max(-1, gid)
	return o
}

// Open a new [Atomic] for writing. Writes to it will be unbuffered. It will be
// backed by a hidden (i.e. its name will start with a dot), temporary file (it
// will have a .tmp extension). If the returned Atomic gets closed without
// calling [Atomic.Finish] before, the temporary file will be deleted without
// touching the target. Use like so:
//
//	f, err := file.AtomicWithTarget("foo").Open()
//	if err != nil {
//		return err
//	}
//	defer f.Close()
//	_, err = f.Write([]byte("I am atomic!"))
//	if err != nil {
//		return err
//	}
//	return f.Finish()
func (o *AtomicOpener) Open() (f *Atomic, err error) {
	f = &Atomic{atomicOpts: o.atomicOpts}

	// Determine the absolute path of the target. This is a safeguard to make
	// the Atomic robust against intermediary working directory changes.
	if f.target, err = filepath.Abs(f.target); err != nil {
		return nil, err
	}

	// This will actually open the file in read/write mode,
	// but we're not going to tell anyone about it.
	f.fd, err = os.CreateTemp(filepath.Dir(f.target), fmt.Sprintf(".%s.*.tmp", filepath.Base(f.target)))
	if err != nil {
		return nil, err
	}

	return f, nil
}

// A writer for atomic file creations or replacements.
type AtomicWriter interface {
	io.Writer
	io.ReaderFrom
}

// Perform the atomic file creation or replacement. The contents of the file
// will be those that the write callback writes to the [AtomicWriter] that gets
// passed in. The writer will be unbuffered. The writer will be backed by a
// hidden (i.e. its name will start with a dot), temporary file (it will have a
// .tmp extension). If write returns without an error, the temporary file will
// be renamed to the target name, otherwise it will be deleted without touching
// the target.
//
// Note that the atomicity aspects are only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func (o *AtomicOpener) Do(write func(unbuffered AtomicWriter) error) (err error) {
	f, err := o.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if err := write(f); err != nil {
		return err
	}
	return f.Finish()
}

// Atomically create or replace the target file with the given content.
// Will delegate to [AtomicOpener.Do].
func (o *AtomicOpener) Write(content []byte) error {
	return o.Do(func(w AtomicWriter) error {
		_, err := w.Write(content)
		return err
	})
}

// Atomically create or replace the target file with the given content.
// Will delegate to [AtomicOpener.Write].
func (o *AtomicOpener) WriteString(content string) error {
	return o.Write([]byte(content))
}

// A file that will appear atomically at its target path after [Atomic.Finish]
// has been called.
//
// Note that the atomicity aspects are only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
type Atomic struct {
	atomicOpts
	fd     *os.File
	closed atomic.Bool
}

func (f *Atomic) Name() string {
	return f.target
}

// Write implements [io.Writer].
func (f *Atomic) Write(p []byte) (int, error) {
	if f == nil {
		return 0, fs.ErrInvalid
	}

	return f.fd.Write(p)
}

// ReadFrom implements [io.ReaderFrom].
func (f *Atomic) ReadFrom(r io.Reader) (int64, error) {
	if f == nil {
		return 0, fs.ErrInvalid
	}

	return f.fd.ReadFrom(r)
}

// Finishes f by closing it and making it appear atomically at its target path.
// The temporary file will be renamed to target, unless finishing fails and an
// error is returned, in which case the temporary file will be deleted without
// touching the target.
//
// Note that the atomicity aspects are only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func (f *Atomic) Finish() (err error) {
	return f.finish(f.target)
}

// Like Finish, but replaces the target base name with the given one.
// Note that the directory cannot be changed, just the file's base name.
func (f *Atomic) FinishWithBaseName(baseName string) (err error) {
	dir := filepath.Dir(f.target)
	target := filepath.Join(dir, baseName)
	if filepath.Base(target) != baseName || filepath.Dir(target) != dir {
		return errors.New("base name is invalid")
	}

	return f.finish(target)
}

func (f *Atomic) finish(target string) (err error) {
	if f == nil {
		return fs.ErrInvalid
	}

	if !f.closed.CompareAndSwap(false, true) {
		return &fs.PathError{Op: "close", Path: f.target, Err: fs.ErrClosed}
	}

	close := true
	defer func() {
		var closeErr, removeErr error
		if close {
			closeErr = f.fd.Close()
		}
		if err != nil {
			removeErr = remove(f.fd)
		}
		err = errors.Join(err, closeErr, removeErr)
	}()

	// https://github.com/google/renameio/blob/v2.0.0/tempfile.go#L150-L157
	if err = f.fd.Sync(); err != nil {
		return err
	}

	close = false // If Close() fails or panics, don't try it a second time.
	if err = f.fd.Close(); err != nil {
		return err
	}

	if f.wantsChmod() {
		if err := os.Chmod(f.fd.Name(), f.permissions.Perm()); err != nil {
			return err
		}
	}

	if f.mtime != (time.Time{}) {
		if err := os.Chtimes(f.fd.Name(), f.mtime, f.mtime); err != nil {
			return err
		}
	}

	// Apply the owner and group changes, if specified. Since chown is a
	// privileged operation (i.e. requires CAP_CHOWN on Linux / root on macOS),
	// it is safe to do this after the permission change. So if this succeeds,
	// the current process itself is privileged, and it's safe to assume that
	// its owner and group are privileged, too. Changing the owner and group
	// information is therefore considered an expansion of access, not a
	// restriction. Doing it the other way round and changing the owner before
	// changing permissions would require yet another capability on Linux for
	// chmod to succeed (CAP_FOWNER).
	if wantsChown := (f.uid >= 0 || f.gid >= 0); wantsChown {
		err = os.Chown(f.fd.Name(), f.uid, f.gid)
		// Ignore errors indicating that os.Chown() is unsupported.
		if err != nil && !errors.Is(err, errors.ErrUnsupported) {
			return err
		}
	}

	if err = os.Rename(f.fd.Name(), target); err != nil {
		return err
	}

	return nil
}

// Closes f and deletes its temporary shadow. The target remains untouched.
// This is a no-op if f has already been finished/closed.
func (f *Atomic) Close() (err error) {
	if f == nil {
		return fs.ErrInvalid
	}

	if !f.closed.CompareAndSwap(false, true) {
		return nil // Already closed, make this a no-op.
	}

	return errors.Join(f.fd.Close(), remove(f.fd))
}

func remove(fd *os.File) error {
	err := os.Remove(fd.Name())
	// Don't propagate any fs.ErrNotExist errors. There is no point in
	// doing this, since the desired state is already reached: The
	// temporary file is no longer present on the file system.
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

// Atomically create or replace a file. The contents of the file will be those
// that the write callback writes to the [io.Writer] that gets passed in. The
// [io.Writer] will be unbuffered. WriteAtomically will buffer the contents in a
// hidden (i.e. its name will start with a dot), temporary file (it will have a
// .tmp extension). When write returns without an error, the temporary file will
// be renamed to fileName, otherwise it will be deleted without touching the
// target file.
//
// Note that the atomicity aspects are only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func WriteAtomically(fileName string, perm os.FileMode, write func(file io.Writer) error) (err error) {
	return AtomicWithTarget(fileName).WithPermissions(perm).Do(func(unbuffered AtomicWriter) error {
		return write(unbuffered)
	})
}

// Atomically create or replace a file with the given content.
// WriteContentAtomically will create a hidden (i.e. its name will start with a
// dot), temporary file (it will have a .tmp extension) with the given content.
// Afterwards, the temporary file will be renamed to fileName unless there was
// an error, in which case the temporary file will be deleted without touching
// the target file.
//
// Note that the atomicity aspects are only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func WriteContentAtomically(fileName string, content []byte, perm os.FileMode) error {
	return AtomicWithTarget(fileName).WithPermissions(perm).Write(content)
}
