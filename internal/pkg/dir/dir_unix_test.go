//go:build unix

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package dir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()

	foo := filepath.Join(tmpDir, "foo")
	require.NoError(t, dir.Init(foo, 0700), "failed to create temp dir foo")

	checkPermissions(t, foo, 0700)

	oldUmask := unix.Umask(0027)
	t.Cleanup(func() { unix.Umask(oldUmask) })

	bar := filepath.Join(tmpDir, "bar")
	require.NoError(t, dir.Init(bar, 0755), "failed to create temp dir bar")
	checkPermissions(t, bar, 0755)
}

// CheckPermissions checks the correct permissions
func checkPermissions(t *testing.T, path string, want os.FileMode) {
	info, err := os.Stat(path)
	require.NoError(t, err, path)
	assert.Equalf(t, want, info.Mode().Perm(), "%s has unexpected permissions", path)
}

func TestPathListJoin(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.Empty(t, dir.PathListJoin())
	})
	t.Run("single", func(t *testing.T) {
		require.Equal(t, "foo", dir.PathListJoin("foo"))
	})
	t.Run("multiple", func(t *testing.T) {
		require.Equal(t, "foo:bar:baz", dir.PathListJoin("foo", "bar", "baz"))
	})
}
