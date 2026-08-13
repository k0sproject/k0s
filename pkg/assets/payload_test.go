// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeExecutableWithPayload writes a file that mimics a k0s executable with a
// ZIP payload appended to it: some opaque prefix bytes standing in for the
// executable image, followed by a ZIP archive containing entries.
func writeExecutableWithPayload(t *testing.T, entries map[string][]byte) string {
	t.Helper()

	var buf bytes.Buffer
	exec, err := os.Executable()
	require.NoError(t, err)
	prefix, err := os.ReadFile(exec)
	require.NoError(t, err)
	_, err = buf.Write(prefix)
	require.NoError(t, err)

	archive := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		// Use CreateHeader instead of Create, so that the modification times
		// stay zero, just like in the payloads produced by hack/zip-files.
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		require.NoError(t, err)
		_, err = entry.Write(entries[name])
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())

	path := filepath.Join(t.TempDir(), "k0s-withpayload")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0755))
	return path
}

func TestOpenPayload_NoPayload(t *testing.T) {
	t.Run("bareExecutable", func(t *testing.T) {
		file, err := os.Executable()
		require.NoError(t, err)

		payload, err := openPayload(file)
		assert.ErrorIs(t, err, ErrNoPayload)
		assert.Nil(t, payload)
	})

	t.Run("emptyFile", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "k0s")
		require.NoError(t, os.WriteFile(path, nil, 0755))

		payload, err := openPayload(path)
		assert.Error(t, err)
		assert.Nil(t, payload)
	})
}

// Failing to inspect an executable must be distinguishable from it not having
// any payload attached, so that such errors don't get mistaken for a bare k0s.
func TestOpenPayload_InaccessibleExecutable(t *testing.T) {
	t.Run("nonExistent", func(t *testing.T) {
		payload, err := openPayload(filepath.Join(t.TempDir(), "enoent"))
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.NotErrorIs(t, err, ErrNoPayload)
		assert.Nil(t, payload)
	})

	t.Run("directory", func(t *testing.T) {
		payload, err := openPayload(t.TempDir())
		assert.ErrorIs(t, err, syscall.EISDIR)
		assert.NotErrorIs(t, err, ErrNoPayload)
		assert.Nil(t, payload)
	})
}

func TestPayload_ModTime(t *testing.T) {
	path := writeExecutableWithPayload(t, map[string][]byte{"bin/etcd": []byte("etcd")})
	modTime := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(path, time.Time{}, modTime))

	payload, err := openPayload(path)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, payload.Close()) })

	assert.True(t, modTime.Equal(payload.ModTime()),
		"Expected the payload to report the executable's modification time %s, got %s",
		modTime, payload.ModTime())
}

// Payloads don't contain any explicit directory entries. The fs.FS
// implementation of archive/zip synthesizes them, which is what allows the
// payload's directories to be enumerated.
func TestPayload_SynthesizedDirectories(t *testing.T) {
	path := writeExecutableWithPayload(t, map[string][]byte{
		"bin/containerd":  []byte("containerd"),
		"bin/etcd":        []byte("etcd"),
		"images/some.tar": []byte("some bundle"),
	})

	payload, err := openPayload(path)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, payload.Close()) })

	t.Run("root", func(t *testing.T) {
		entries, err := fs.ReadDir(payload, ".")
		require.NoError(t, err)
		names := make([]string, len(entries))
		for i, entry := range entries {
			assert.True(t, entry.IsDir(), "Expected %s to be a directory", entry.Name())
			names[i] = entry.Name()
		}
		assert.Equal(t, []string{"bin", "images"}, names)
	})

	t.Run("bin", func(t *testing.T) {
		entries, err := fs.ReadDir(payload, EmbeddedBinDir)
		require.NoError(t, err)
		names := make([]string, len(entries))
		for i, entry := range entries {
			assert.False(t, entry.IsDir(), "Expected %s to be a file", entry.Name())
			names[i] = entry.Name()
		}
		assert.Equal(t, []string{"containerd", "etcd"}, names)
	})
}

func TestContentID(t *testing.T) {
	contentID := func(t *testing.T, payload *Payload, name string) (string, bool) {
		info, err := fs.Stat(payload, name)
		require.NoError(t, err)
		return ContentID(info)
	}

	openPayloadWith := func(t *testing.T, entries map[string][]byte) *Payload {
		payload, err := openPayload(writeExecutableWithPayload(t, entries))
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, payload.Close()) })
		return payload
	}

	t.Run("identifiesContents", func(t *testing.T) {
		payload := openPayloadWith(t, map[string][]byte{
			"images/foo.tar": []byte("some bundle"),
			"images/bar.tar": []byte("another bundle"),
		})

		foo, ok := contentID(t, payload, "images/foo.tar")
		require.True(t, ok)
		assert.NotEmpty(t, foo)
		bar, ok := contentID(t, payload, "images/bar.tar")
		require.True(t, ok)
		assert.NotEqual(t, foo, bar, "Entries with differing contents should be distinguishable")
	})

	// Identifiers have to survive rebuilds of the executable, so that unchanged
	// bundles won't be imported again after an upgrade.
	t.Run("stableAcrossExecutables", func(t *testing.T) {
		entries := map[string][]byte{"images/foo.tar": []byte("some bundle")}
		first := openPayloadWith(t, entries)
		second := openPayloadWith(t, entries)

		firstID, ok := contentID(t, first, "images/foo.tar")
		require.True(t, ok)
		secondID, ok := contentID(t, second, "images/foo.tar")
		require.True(t, ok)
		assert.Equal(t, firstID, secondID)
	})

	t.Run("unidentifiable", func(t *testing.T) {
		payload := openPayloadWith(t, map[string][]byte{
			"bin/containerd":   []byte("containerd"),
			"images/empty.tar": nil,
		})

		// Directories are synthesized, so they have no recorded contents.
		t.Run("directory", func(t *testing.T) {
			id, ok := contentID(t, payload, EmbeddedBinDir)
			assert.False(t, ok)
			assert.Empty(t, id)
		})

		t.Run("emptyEntry", func(t *testing.T) {
			id, ok := contentID(t, payload, "images/empty.tar")
			assert.False(t, ok)
			assert.Empty(t, id)
		})

		// FileInfos that don't originate from a payload can't be identified.
		t.Run("foreignFileInfo", func(t *testing.T) {
			info, err := os.Stat(t.TempDir())
			require.NoError(t, err)
			id, ok := ContentID(info)
			assert.False(t, ok)
			assert.Empty(t, id)
		})
	})
}

func TestPayload_Open(t *testing.T) {
	path := writeExecutableWithPayload(t, map[string][]byte{
		"bin/containerd": []byte("containerd"),
		"bin/etcd":       []byte("etcd"),
	})

	payload, err := openPayload(path)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, payload.Close()) })

	t.Run("notExist", func(t *testing.T) {
		file, err := payload.Open("bin/nonexistent")
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Nil(t, file)
	})

	// Executables are looked up by their exact names. Payload entries are not
	// reachable by any other name, in particular not without their directory.
	t.Run("noLookupByBaseName", func(t *testing.T) {
		file, err := payload.Open("containerd")
		assert.ErrorIs(t, err, fs.ErrNotExist)
		assert.Nil(t, file)
	})

	t.Run("multipleOpenEntriesConcurrently", func(t *testing.T) {
		containerd, err := payload.Open("bin/containerd")
		require.NoError(t, err)
		defer func() { assert.NoError(t, containerd.Close()) }()
		etcd, err := payload.Open("bin/etcd")
		require.NoError(t, err)
		defer func() { assert.NoError(t, etcd.Close()) }()

		etcdContents, err := io.ReadAll(etcd)
		require.NoError(t, err)
		assert.Equal(t, "etcd", string(etcdContents))
		containerdContents, err := io.ReadAll(containerd)
		require.NoError(t, err)
		assert.Equal(t, "containerd", string(containerdContents))
	})
}
