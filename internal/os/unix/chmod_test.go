//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package unix_test

import (
	"os"
	"path/filepath"
	"testing"

	osunix "github.com/k0sproject/k0s/internal/os/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPath_Chmod(t *testing.T) {
	testPathChmod(t, os.O_RDONLY)
}

func testPathChmod(t *testing.T, flags int) {
	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(path, nil, 0600))

	p, err := osunix.OpenPath(path, flags, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, p.Close()) })

	require.NoError(t, p.Chmod(0640), "chmod via the path handle should succeed")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm(), "the file's mode should have been changed")
}
