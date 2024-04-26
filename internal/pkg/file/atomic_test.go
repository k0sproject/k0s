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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"syscall"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAtomically(t *testing.T) {

	t.Run("filePermissions", func(t *testing.T) {
		for _, mode := range []struct{ posix, win os.FileMode }{
			// On Windows, file mode just mimics the read-only flag
			{0400, 0444}, {0755, 0666}, {0644, 0666}, {0777, 0666},
		} {
			modeStr := strconv.FormatUint(uint64(mode.posix), 8)
			t.Run(modeStr, func(t *testing.T) {
				dir := t.TempDir()
				file := filepath.Join(dir, "file")

				require.NoError(t, WriteAtomically(file, mode.posix, func(file io.Writer) error {
					_, err := file.Write([]byte(modeStr))
					return err
				}))

				content, err := os.ReadFile(file)
				require.NoError(t, err)
				assert.Equal(t, []byte(modeStr), content)
				info, err := os.Stat(file)
				if assert.NoError(t, err) {
					expectedMode := mode.posix
					if runtime.GOOS == "windows" {
						expectedMode = mode.win
					}

					assert.Equal(t, expectedMode, info.Mode())
				}
			})
		}
	})

	// Several tests about error handling and reporting. Some of them are a bit
	// contrived and expect the Writer to be a pointer to Atomic in order to
	// break things in ways usual consumers of the API wouldn't be able to do,
	// as the interface doesn't make any guarantees about the actual type of the
	// Writer.

	assertPathError := func(t *testing.T, err error, op, dir string) (pathErr *os.PathError, ok bool) {
		t.Helper()
		if ok = assert.True(t, errors.As(err, &pathErr), "Not a PathError: %v", err); ok {
			assert.Equal(t, op, pathErr.Op)
			assert.Equal(
				t, dir, filepath.Dir(pathErr.Path),
				"Expected the temporary file to be in the same directory as the target file",
			)
		}
		return
	}

	assertDirEmpty := func(t *testing.T, dir string) bool {
		t.Helper()
		entries, err := os.ReadDir(dir)
		return assert.NoError(t, err) && assert.Empty(t, entries)
	}

	t.Run("writeFails", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")
		err := WriteAtomically(file, 0644, func(file io.Writer) error {
			return assert.AnError
		})
		if errs := flatten(err); assert.Len(t, errs, 1) {
			assert.Same(t, assert.AnError, errs[0])
		}
		assertDirEmpty(t, dir)
	})

	t.Run("writePanics", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		defer func() {
			assert.Same(t, assert.AnError, recover())
			assertDirEmpty(t, dir)
		}()

		_ = WriteAtomically(file, 0644, func(io.Writer) error { panic(assert.AnError) })
		assert.Fail(t, "Should have panicked!")
	})

	t.Run("workingDirectoryChanges", func(t *testing.T) {
		dir := t.TempDir()
		otherDir := t.TempDir()
		defer testutil.Chdir(t, dir)()

		assert.NoError(t, WriteAtomically("file", 0644, func(w io.Writer) error {
			assert.NoError(t, os.Chdir(otherDir))
			return nil
		}))
		assertDirEmpty(t, otherDir)
		assert.FileExists(t, filepath.Join(dir, "file"))
	})

	t.Run("tempFileClosed", func(t *testing.T) {
		// Prove that multiple errors may be reported.
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		errs := flatten(WriteAtomically(file, 0644, func(file io.Writer) error {
			a, ok := file.(*Atomic)
			require.True(t, ok, "Not an Atomic: %T", file)
			require.NoError(t, a.fd.Close())
			return nil
		}))

		require.Len(t, errs, 2)
		var tempPath string

		// The first error should be about the failed attempt to sync the temporary file.
		if err, ok := assertPathError(t, errs[0], "sync", dir); ok {
			tempPath = err.Path
			assert.True(t, errors.Is(err, fs.ErrClosed), "Expected fs.ErrClosed: %v", err.Err)
		}

		// The second error should be about the failed attempt to close the temporary file.
		if err, ok := assertPathError(t, errs[1], "close", dir); ok {
			if tempPath != "" {
				assert.Equal(t, tempPath, err.Path, "Temp paths differ between errors")
			}
			assert.True(t, errors.Is(err, fs.ErrClosed), "Expected fs.ErrClosed: %v", err.Err)
		}

		assertDirEmpty(t, dir)
	})

	t.Run("tempFileRemoved", func(t *testing.T) {
		// Prove that any fs.ErrNotExist removal errors are not propagated.
		// There is no point in doing this, since the desired state is already
		// reached: The temporary file is no longer present on the file system.
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		var tempPath string
		err := WriteAtomically(file, 0644, func(file io.Writer) error {
			a, ok := file.(*Atomic)
			require.True(t, ok, "Not an Atomic: %T", file)
			tempPath = a.fd.Name()
			require.Equal(t, dir, filepath.Dir(tempPath))
			if runtime.GOOS == "windows" {
				t.Skip("Cannot remove a file which is still opened on Windows")
			}
			require.NoError(t, os.Remove(tempPath))
			return nil
		})

		assert.Len(t, flatten(err), 1)

		// The error should be about the failed chmod.
		if err, ok := assertPathError(t, err, "chmod", dir); ok {
			assert.Equal(t, tempPath, err.Path, "Error refers to unexpected path")
			assert.True(t, errors.Is(err, fs.ErrNotExist), "Expected fs.ErrNotExist: %v", err.Err)
		}

		assertDirEmpty(t, dir)
	})

	t.Run("pathObstructed", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		// Obstruct the file path, so that the rename fails.
		require.NoError(t, os.Mkdir(file, 0700))

		errs := flatten(WriteAtomically(file, 0644, func(file io.Writer) error {
			_, err := file.Write([]byte("obstructed"))
			return err
		}))

		require.Len(t, errs, 1)

		var linkErr *os.LinkError
		if assert.True(t, errors.As(errs[0], &linkErr), "Not a LinkError: %#+v", errs[0]) {
			assert.Equal(t, "rename", linkErr.Op)
			assert.Equal(
				t, dir, filepath.Dir(linkErr.Old),
				"Expected the temporary file to be in the same directory as the target file",
			)
			assert.Equal(t, file, linkErr.New)
			if runtime.GOOS == "windows" {
				// https://github.com/golang/go/blob/go1.20/src/syscall/types_windows.go#L11
				//revive:disable-next-line:var-naming
				const ERROR_ACCESS_DENIED syscall.Errno = 5
				var errno syscall.Errno
				ok := errors.As(linkErr.Err, &errno)
				ok = ok && errno == ERROR_ACCESS_DENIED
				assert.True(t, ok, "Expected ERROR_ACCESS_DENIED: %v", linkErr.Err)
			} else {
				assert.True(t, errors.Is(linkErr.Err, fs.ErrExist), "Expected fs.ErrExist: %v", linkErr.Err)
			}
		}

		// Expect just the single directory that was created in order to obstruct the file path.
		if entries, err := os.ReadDir(dir); assert.NoError(t, err) && assert.Len(t, entries, 1) {
			e := entries[0]
			name := e.Name()
			assert.Equal(t, filepath.Base(file), name)
			assert.True(t, e.IsDir(), "Not a directory: %s", name)
		}
	})

	t.Run("tempPathObstructed", func(t *testing.T) {
		// Prove that any non fs.ErrNotExist removal errors are propagated correctly.
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		// Obstruct the file path, so that the rename fails.
		require.NoError(t, os.Mkdir(file, 0700))

		var tempPath string
		errs := flatten(WriteAtomically(file, 0755, func(file io.Writer) error {
			a, ok := file.(*Atomic)
			require.True(t, ok, "Not an Atomic: %T", file)
			tempPath = a.fd.Name()
			require.Equal(t, dir, filepath.Dir(tempPath))

			if runtime.GOOS == "windows" {
				t.Skip("Cannot remove a file which is still opened on Windows")
			}

			// Remove the temporary file ...
			require.NoError(t, os.Remove(tempPath))
			// ... obstruct the temporary file path ...
			require.NoError(t, os.Mkdir(tempPath, 0700))
			// .. and ensure that the directory can't be removed.
			require.NoError(t, os.WriteFile(filepath.Join(tempPath, ".keep"), []byte{}, 0600))

			return nil
		}))

		// The outcome here is a bit weird, since the chmod will actually
		// succeed, but instead it will chmod the directory, not the file. So
		// there's no chmod error expected. But the tests will fail if the file
		// mode given to WriteAtomically won't have the executable bit set,
		// since the automatic temporary directory cleanup won't handle this.

		require.Len(t, errs, 2)

		// The first error should be about the failed rename.
		var linkErr *os.LinkError
		if assert.True(t, errors.As(errs[0], &linkErr), "Not a LinkError: %v", linkErr) {
			assert.Equal(t, "rename", linkErr.Op)
			assert.Equal(t, tempPath, linkErr.Old, "Error refers to unexpected path")
			assert.Equal(t, file, linkErr.New)
			assert.True(t, errors.Is(linkErr.Err, fs.ErrExist), "Expected fs.ErrExist: %v", linkErr.Err)
		}

		// The second error should be about the failed removal.
		if err, ok := assertPathError(t, errs[1], "remove", dir); ok {
			assert.Equal(t, tempPath, err.Path, "Error refers to unexpected path")
			assert.True(t, errors.Is(err, fs.ErrExist), "Expected fs.ErrExist: %v", err.Err)
		}

		// Expect to see two directories left behind.
		if entries, err := os.ReadDir(dir); assert.NoError(t, err) && assert.Len(t, entries, 2) {
			for _, e := range entries {
				assert.True(t, e.IsDir(), "Not a directory: %s", e.Name())
			}
		}
	})
}

func flatten(err error) []error {
	errs := []error{err}

	for i := 0; i < len(errs); {
		if wrapped, ok := errs[i].(interface{ Unwrap() []error }); ok {
			if unwrapped := wrapped.Unwrap(); len(unwrapped) > 0 {
				errs = slices.Replace(errs, i, i+1, unwrapped...)
				continue
			}
		}
		i++
	}

	return errs
}
