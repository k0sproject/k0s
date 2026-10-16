// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStageExecutable_Fallbacks(t *testing.T) {
	stageDir := t.TempDir()
	pathDir := t.TempDir()

	exeName := "some-helper"
	exePath := filepath.Join(pathDir, exeName+constant.ExecutableSuffix)

	t.Setenv("PATH", pathDir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")

	require.NoError(t, os.WriteFile(exePath, nil, 0755))

	t.Run("FromPATH", func(t *testing.T) {
		stagedPath, err := StageExecutable(stageDir, exeName)
		if assert.NoError(t, err) {
			assert.Equal(t, exePath, stagedPath, "Executable should have been looked up from PATH")
		}
	})

	exePath = filepath.Join(stageDir, exeName+constant.ExecutableSuffix)
	require.NoError(t, os.WriteFile(exePath, nil, 0755))

	t.Run("FromDisk", func(t *testing.T) {
		stagedPath, err := StageExecutable(stageDir, exeName)
		if assert.NoError(t, err) {
			assert.Equal(t, exePath, stagedPath, "Executable should have been found on disk")
		}
	})
}

func TestStageFrom(t *testing.T) {
	openTestPayload := func(t *testing.T) *Payload {
		path := writeExecutableWithPayload(t, map[string][]byte{
			"bin/containerd":   []byte("containerd"),
			"bin/nested/thing": []byte("nested thing"),
			"images/foo.tar":   []byte("some bundle"),
		})
		payload, err := openPayload(path)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, payload.Close()) })
		return payload
	}

	t.Run("extracts", func(t *testing.T) {
		payload := openTestPayload(t)
		path := filepath.Join(t.TempDir(), "containerd")

		require.NoError(t, stageFrom(payload, "containerd", path, 0750))

		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "containerd", string(contents))

		info, err := os.Stat(path)
		require.NoError(t, err)
		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0750), info.Mode().Perm())
		}
		// Staged files inherit the executable's modification time, so that
		// stageFrom can tell if they are up to date.
		assert.True(t, payload.ModTime().Equal(info.ModTime()),
			"Expected modification time %s, got %s", payload.ModTime(), info.ModTime())
	})

	t.Run("notEmbedded", func(t *testing.T) {
		payload := openTestPayload(t)
		dir := t.TempDir()

		t.Run("noSuchEntry", func(t *testing.T) {
			err := stageFrom(payload, "kubelet", filepath.Join(dir, "kubelet"), 0750)
			assert.ErrorIs(t, err, errNotEmbedded)
		})

		t.Run("directoryEntry", func(t *testing.T) {
			err := stageFrom(payload, "nested", filepath.Join(dir, "nested"), 0750)
			assert.ErrorIs(t, err, errNotEmbedded)
		})

		// Asset names are not supposed to be paths. Names that would escape the
		// payload's bin directory are rejected outright, instead of being
		// reported as not embedded, which would trigger a fallback to an
		// executable of that name on disk or in the PATH.
		t.Run("nameEscapingBinDir", func(t *testing.T) {
			err := stageFrom(payload, "../images/foo.tar", filepath.Join(dir, "foo.tar"), 0750)
			assert.ErrorIs(t, err, fs.ErrInvalid)
			assert.NotErrorIs(t, err, errNotEmbedded)
		})

		for _, name := range []string{"kubelet", "nested", "foo.tar"} {
			assert.NoFileExists(t, filepath.Join(dir, name))
		}
	})

	// Staging is skipped if the file on disk has both the executable's
	// modification time and the size of the embedded file.
	t.Run("reusesUpToDateFile", func(t *testing.T) {
		payload := openTestPayload(t)
		path := filepath.Join(t.TempDir(), "containerd")
		require.NoError(t, os.WriteFile(path, []byte("kept as-is"), 0750))
		require.NoError(t, os.Chtimes(path, time.Time{}, payload.ModTime()))

		require.NoError(t, stageFrom(payload, "containerd", path, 0750))

		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "kept as-is", string(contents), "Existing file should not have been overwritten")
	})

	t.Run("reStages", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			contents string
			modTime  func(*Payload) time.Time
		}{
			// Same size, but a different modification time.
			{"onModTimeMismatch", "CONTAINERD", func(p *Payload) time.Time {
				return p.ModTime().Add(-time.Hour)
			}},
			// Same modification time, but a different size.
			{"onSizeMismatch", "containerd, but longer", (*Payload).ModTime},
		} {
			t.Run(test.name, func(t *testing.T) {
				payload := openTestPayload(t)
				path := filepath.Join(t.TempDir(), "containerd")
				require.NoError(t, os.WriteFile(path, []byte(test.contents), 0750))
				require.NoError(t, os.Chtimes(path, time.Time{}, test.modTime(payload)))

				require.NoError(t, stageFrom(payload, "containerd", path, 0750))

				contents, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, "containerd", string(contents), "Existing file should have been overwritten")
			})
		}
	})
}
