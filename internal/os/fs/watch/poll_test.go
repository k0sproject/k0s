// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffDirEntries(t *testing.T) {
	t.Run("staleDirEntry", func(t *testing.T) {
		// This might theoretically happen on Linux, where DirEntries don't
		// include file infos. On Windows, reading directory entries already
		// includes file infos, no further syscall is needed, and hence
		// DirEntry.Info() is infallible.

		dirEntries := []os.DirEntry{
			nonExistingDirEntry{},
		}

		diffEntries, err := diffDirEntries(dirEntries, nil, HandlerFunc(func(e Event) {
			assert.Fail(t, "Unexpected event", "%#v", e)
		}))
		require.NoError(t, err)
		assert.Empty(t, diffEntries)
	})
}

type nonExistingDirEntry struct{}

func (nonExistingDirEntry) Name() string               { return "" }
func (nonExistingDirEntry) IsDir() bool                { return false }
func (nonExistingDirEntry) Type() fs.FileMode          { return 0 }
func (nonExistingDirEntry) Info() (fs.FileInfo, error) { return nil, os.ErrNotExist }
