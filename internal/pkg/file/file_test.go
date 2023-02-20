/*
Copyright 2021 k0s authors

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
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
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
	// contrived and expect the Writer to be a File pointer in order to break
	// things in ways usual consumers of the API wouldn't be able to do, as the
	// interface doesn't make any guarantees about the actual type of the
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
		assert.Same(t, assert.AnError, WriteAtomically(file, 0644, func(file io.Writer) error {
			return assert.AnError
		}))
		assertDirEmpty(t, dir)
	})

	t.Run("tempFileClosed", func(t *testing.T) {
		// Prove that multiple errors may be reported.
		dir := t.TempDir()
		file := filepath.Join(dir, "file")

		errs := multierr.Errors(WriteAtomically(file, 0644, func(file io.Writer) error {
			c, ok := file.(io.Closer)
			require.True(t, ok, "Not closeable: %T", file)
			require.NoError(t, c.Close())
			return nil
		}))

		assert.Len(t, errs, 2)
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
			n, ok := file.(interface{ Name() string })
			require.True(t, ok, "Doesn't have a name: %T", file)
			tempPath = n.Name()
			require.Equal(t, dir, filepath.Dir(tempPath))
			if runtime.GOOS == "windows" {
				t.Skip("Cannot remove a file which is still opened on Windows")
			}
			require.NoError(t, os.Remove(tempPath))
			return nil
		})

		assert.Len(t, multierr.Errors(err), 1)

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

		errs := multierr.Errors(WriteAtomically(file, 0644, func(file io.Writer) error {
			_, err := file.Write([]byte("obstructed"))
			return err
		}))

		require.Len(t, errs, 1)

		var linkErr *os.LinkError
		if assert.True(t, errors.As(errs[0], &linkErr), "Not a LinkError: %v", linkErr) {
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
		errs := multierr.Errors(WriteAtomically(file, 0755, func(file io.Writer) error {
			n, ok := file.(interface{ Name() string })
			require.True(t, ok, "Doesn't have a name: %T", file)
			tempPath = n.Name()
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

func TestExists(t *testing.T) {

	t.Run("nonExisting", func(t *testing.T) {
		got := Exists(filepath.Join(t.TempDir(), "non-existing"))
		want := false
		if got != want {
			t.Errorf("test non-existing: got %t, wanted %t", got, want)
		}
	})

	t.Run("existing", func(t *testing.T) {
		existingFileName := filepath.Join(t.TempDir(), "existing")
		require.NoError(t, os.WriteFile(existingFileName, []byte{}, 0644))

		got := Exists(existingFileName)
		want := true
		if got != want {
			t.Errorf("test existing tempfile %s: got %t, wanted %t", existingFileName, got, want)
		}
	})

	t.Run("permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("No UNIX-style permissions on Windows")
		}

		// test what happens when we don't have permissions to the directory to file
		// and can confirm that it actually exists
		dir := t.TempDir()
		existingFileName := filepath.Join(t.TempDir(), "existing")
		if assert.NoError(t, os.Chmod(dir, 0000)) {
			t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
		}

		got := Exists(existingFileName)
		want := false
		if got != want {
			t.Errorf("test existing tempfile %s: got %t, wanted %t", existingFileName, got, want)
		}
	})
}
