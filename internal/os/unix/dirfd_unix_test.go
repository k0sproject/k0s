//go:build unix

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

package unix_test

import (
	"io"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	osunix "github.com/k0sproject/k0s/internal/os/unix"
	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirFD_NotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foo")

	d, err := osunix.OpenDir(path, 0)
	if err == nil {
		assert.NoError(t, d.Close())
	}
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDirFD_Empty(t *testing.T) {
	path := t.TempDir()

	d, err := osunix.OpenDir(path, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	foo := "foo"
	assertENOENT := func(t *testing.T, op string, err error) {
		var pathErr *os.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, op, pathErr.Op)
			assert.Equal(t, foo, pathErr.Path)
			assert.Equal(t, syscall.ENOENT, pathErr.Err)
		}
	}

	_, err = d.OpenDir(foo, 0)
	assertENOENT(t, "openat", err)

	_, err = d.StatAt(foo, 0)
	assertENOENT(t, "fstatat", err)

	err = d.Remove(foo)
	assertENOENT(t, "unlinkat", err)

	err = d.RemoveDir(foo)
	assertENOENT(t, "unlinkat", err)

	if entries, err := d.Readdirnames(1); assert.Equal(t, io.EOF, err) {
		assert.Empty(t, entries)
	}
}

func TestDirFD_Mkdir(t *testing.T) {
	path := t.TempDir()

	d, err := osunix.OpenDir(path, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	require.NoError(t, os.WriteFile(filepath.Join(path, "foo"), []byte("lorem"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(path, "bar"), 0755))

	err = d.Mkdir("foo", 0755)
	if pathErr := (*os.PathError)(nil); assert.ErrorAs(t, err, &pathErr) {
		assert.Equal(t, "mkdirat", pathErr.Op)
		assert.Equal(t, "foo", pathErr.Path)
		assert.Equal(t, syscall.EEXIST, pathErr.Err)
	}

	err = d.Mkdir("bar", 0755)
	if pathErr := (*os.PathError)(nil); assert.ErrorAs(t, err, &pathErr) {
		assert.Equal(t, "mkdirat", pathErr.Op)
		assert.Equal(t, "bar", pathErr.Path)
		assert.Equal(t, syscall.EEXIST, pathErr.Err)
	}

	err = d.Mkdir("baz", 0755)
	if assert.NoError(t, err) {
		stat, err := os.Stat(filepath.Join(path, "baz"))
		if assert.NoError(t, err) {
			assert.Equal(t, os.FileMode(0755), stat.Mode()&os.ModePerm)
			assert.True(t, stat.IsDir())
		}
	}
}

func TestDirFD_Filled(t *testing.T) {
	dirPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dirPath, "foo"), []byte("lorem"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dirPath, "bar"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dirPath, "bar", "baz"), []byte("ipsum"), 0400))

	now := time.Now()
	require.NoError(t, os.Chtimes(filepath.Join(dirPath, "foo"), time.Time{}, now.Add(-3*time.Minute)))
	require.NoError(t, os.Chtimes(filepath.Join(dirPath, "bar", "baz"), time.Time{}, now.Add(-2*time.Minute)))
	require.NoError(t, os.Chtimes(filepath.Join(dirPath, "bar"), time.Time{}, now.Add(-1*time.Minute)))

	d, err := osunix.OpenDir(dirPath, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	// Stat foo and match contents.
	if stat, err := d.StatAt("foo", 0); assert.NoError(t, err) {
		assert.Equal(t, "foo", stat.Name())
		assert.Equal(t, int64(5), stat.Size())
		assert.WithinDuration(t, now.Add(-3*time.Minute), stat.ModTime(), 0)
		assert.Equal(t, os.FileMode(0644), stat.Mode())
		assert.False(t, stat.IsDir())
		assert.IsType(t, new(unix.Stat_t), stat.Sys())
	}

	// Stat bar and match contents.
	if stat, err := d.StatAt("bar", 0); assert.NoError(t, err) {
		assert.Equal(t, "bar", stat.Name())
		assert.Greater(t, stat.Size(), int64(0))
		assert.WithinDuration(t, now.Add(-1*time.Minute), stat.ModTime(), 0)
		assert.Equal(t, os.FileMode(0755)|os.ModeDir, stat.Mode())
		assert.True(t, stat.IsDir())
		assert.IsType(t, new(unix.Stat_t), stat.Sys())
	}

	// Stat bar/baz and match contents.
	if stat, err := d.StatAt(filepath.Join("bar", "baz"), 0); assert.NoError(t, err) {
		assert.Equal(t, "baz", stat.Name())
		assert.Equal(t, int64(5), stat.Size())
		assert.WithinDuration(t, now.Add(-2*time.Minute), stat.ModTime(), 0)
		assert.Equal(t, os.FileMode(0400), stat.Mode())
		assert.False(t, stat.IsDir())
		assert.IsType(t, new(unix.Stat_t), stat.Sys())
	}

	// List directory contents and match for correctness.
	entries, err := d.Readdirnames(10)
	if assert.NoError(t, err) && assert.Len(t, entries, 2) {
		assert.ElementsMatch(t, entries, []string{"foo", "bar"})
	}
	entries, err = d.Readdirnames(10)
	assert.Empty(t, entries)
	assert.Same(t, io.EOF, err)
}

func TestDirFD_Entries(t *testing.T) {
	dirPath, expectedNames := t.TempDir(), make([]string, 10)
	for i := range expectedNames {
		expectedNames[i] = strconv.Itoa(i)
		require.NoError(t, os.WriteFile(filepath.Join(dirPath, expectedNames[i]), nil, 0644))
	}

	d, err := osunix.OpenDir(dirPath, 0)
	require.NoError(t, err)
	close := sync.OnceFunc(func() { assert.NoError(t, d.Close()) })
	t.Cleanup(close)

	var names []string
	for name, err := range d.ReadEntryNames() {
		require.NoError(t, err)
		names = append(names, name)
		if len(names) >= len(expectedNames)/2 {
			break // test early break
		}
	}
	for name, err := range d.ReadEntryNames() {
		require.NoError(t, err)
		names = append(names, name)
	}

	assert.ElementsMatch(t, expectedNames, names)

	for range d.ReadEntryNames() {
		require.Fail(t, "Shouldn't yield any additional entries after a full iteration")
	}

	close()
	next, stop := iter.Pull2(d.ReadEntryNames())
	defer stop()
	if name, err, hasNext := next(); assert.True(t, hasNext, "Should yield exactly one error") {
		assert.Zero(t, name)
		assert.ErrorContains(t, err, "use of closed file")
	}
	_, _, hasNext := next()
	assert.False(t, hasNext, "Should yield exactly one error")
}
