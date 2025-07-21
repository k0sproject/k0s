// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package dir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/stretchr/testify/require"
)

func TestGetAll(t *testing.T) {
	t.Run("empty", func(t *testing.T) {

		tmpDir := t.TempDir()
		dirs, err := dir.GetAll(tmpDir)
		require.NoError(t, err)
		require.Empty(t, dirs)
	})
	t.Run("filter dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, dir.Init(filepath.Join(tmpDir, "dir1"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1"), []byte{}, 0600))
		dirs, err := dir.GetAll(tmpDir)
		require.NoError(t, err)
		require.Equal(t, []string{"dir1"}, dirs)
	})
}
