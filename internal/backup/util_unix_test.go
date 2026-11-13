//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	internalio "github.com/k0sproject/k0s/internal/io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateArchive(t *testing.T) {
	t.Run("fails when the archive cannot be written completely", func(t *testing.T) {
		baseDir := t.TempDir()
		path := filepath.Join(baseDir, "some-file")
		require.NoError(t, os.WriteFile(path, []byte("some data"), 0644))
		files := []string{path}

		var reference bytes.Buffer
		require.NoError(t, createArchive(&reference, files, baseDir))

		// Simulate an error at every possible write position.
		for capacity := range reference.Len() {
			remaining := capacity
			archive := internalio.WriterFunc(func(p []byte) (int, error) {
				l := len(p)
				if l > remaining {
					written := remaining
					remaining = 0
					return written, assert.AnError
				}

				remaining -= l
				return l, nil
			})

			err := createArchive(archive, files, baseDir)
			require.ErrorIsf(t, err, assert.AnError,
				"Expected the injected error after %d of %d bytes had been written",
				capacity, reference.Len(),
			)
		}
	})
}
