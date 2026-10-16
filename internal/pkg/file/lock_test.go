// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	holder, err := file.LockPath(path, true)
	require.NoError(t, err)

	other, err := os.Open(path)
	require.NoError(t, err)
	defer other.Close()

	// Another file holds the exclusive lock, so a shared one isn't possible.
	locked, err := file.TryLock(other, false)
	require.NoError(t, err)
	assert.False(t, locked)

	// Closing the holder releases the lock.
	require.NoError(t, holder.Close())
	locked, err = file.TryLock(other, false)
	require.NoError(t, err)
	assert.True(t, locked)
}
