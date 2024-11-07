//go:build unix

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	osunix "github.com/k0sproject/k0s/internal/os/unix"
	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir_NotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foo")

	d, err := osunix.OpenDir(path, 0)
	if err == nil {
		assert.NoError(t, d.Close())
	}
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDir_Empty(t *testing.T) {
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

	_, err = d.Open(foo)
	assertENOENT(t, "openat", err)

	_, err = fs.Stat(d, foo)
	assertENOENT(t, "fstatat", err)

	_, err = fs.ReadFile(d, foo)
	assertENOENT(t, "openat", err)

	var pathErr *os.PathError

	// Reading a directory as a file should yield the right error.
	if _, err = fs.ReadFile(d, "."); assert.ErrorAs(t, err, &pathErr) {
		assert.Equal(t, "read", pathErr.Op)
		assert.Equal(t, ".", pathErr.Path)
		assert.Equal(t, syscall.EISDIR, pathErr.Err)
	}

	// We don't want to allow reading directories via io/fs.
	if _, err = fs.ReadDir(d, "."); assert.ErrorAs(t, err, &pathErr) {
		assert.Equal(t, "readdir", pathErr.Op)
		assert.Equal(t, ".", pathErr.Path)
		assert.ErrorContains(t, pathErr.Err, "not implemented")
	}

	if entries, err := d.Readdirnames(1); assert.Equal(t, io.EOF, err) {
		assert.Empty(t, entries)
	}
}

func TestDir_Filled(t *testing.T) {
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

	// Read foo and match contents.
	if data, err := fs.ReadFile(d, "foo"); assert.NoError(t, err) {
		assert.Equal(t, []byte("lorem"), data)
	}

	// Stat foo and match contents.
	if stat, err := fs.Stat(d, "foo"); assert.NoError(t, err) {
		assert.Equal(t, "foo", stat.Name())
		assert.Equal(t, int64(5), stat.Size())
		assert.WithinDuration(t, now.Add(-3*time.Minute), stat.ModTime(), 0)
		assert.Equal(t, os.FileMode(0644), stat.Mode())
		assert.False(t, stat.IsDir())
		assert.IsType(t, new(unix.Stat_t), stat.Sys())
	}

	// Stat bar and match contents.
	if stat, err := fs.Stat(d, "bar"); assert.NoError(t, err) {
		assert.Equal(t, "bar", stat.Name())
		assert.Positive(t, stat.Size(), int64(0))
		assert.WithinDuration(t, now.Add(-1*time.Minute), stat.ModTime(), 0)
		assert.Equal(t, os.FileMode(0755)|os.ModeDir, stat.Mode())
		assert.True(t, stat.IsDir())
		assert.IsType(t, new(unix.Stat_t), stat.Sys())
	}

	// Stat bar/baz and match contents.
	if stat, err := fs.Stat(d, filepath.Join("bar", "baz")); assert.NoError(t, err) {
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

	// Read bar/baz and match contents.
	if data, err := fs.ReadFile(d, filepath.Join("bar", "baz")); assert.NoError(t, err) {
		assert.Equal(t, []byte("ipsum"), data)
	}
}
