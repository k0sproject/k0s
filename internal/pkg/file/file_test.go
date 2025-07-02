// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
