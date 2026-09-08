// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package download

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveContent(t *testing.T, content string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestDownload_ReturnsPath(t *testing.T) {
	dir := t.TempDir()
	url := serveContent(t, "k0s-binary")

	path, err := NewDownloader(Config{
		URL:         url,
		DownloadDir: dir,
		Filename:    "k0s.tmp",
	}).Download(t.Context())

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "k0s.tmp"), path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "k0s-binary", string(content))
}

// TestDownload_ReturnsAbsolutePathForRelativeDir pins the returned path to where
// the file lands. The writer resolves its target absolutely to survive a working
// directory change, so a path rebuilt from a relative DownloadDir would dangle.
func TestDownload_ReturnsAbsolutePathForRelativeDir(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	require.NoError(t, os.Mkdir("downloads", 0o700))

	path, err := NewDownloader(Config{
		URL:         serveContent(t, "k0s-binary"),
		DownloadDir: "downloads",
		Filename:    "k0s.tmp",
	}).Download(t.Context())

	require.NoError(t, err)
	require.True(t, filepath.IsAbs(path), "expected an absolute path, got %q", path)
	assert.Equal(t, filepath.Join(work, "downloads", "k0s.tmp"), path)

	// The path has to keep resolving once the working directory moves.
	t.Chdir(t.TempDir())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "k0s-binary", string(content))
}

func TestDownload_RejectsFilenameWithPathElements(t *testing.T) {
	path, err := NewDownloader(Config{
		URL:         serveContent(t, "whatever"),
		DownloadDir: t.TempDir(),
		Filename:    "nested/k0s.tmp",
	}).Download(t.Context())

	assert.ErrorContains(t, err, "filename contains path elements")
	assert.Empty(t, path)
}

func TestDownload_HashMismatchLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "k0s.tmp")

	// A previous download must survive a failed attempt.
	require.NoError(t, os.WriteFile(target, []byte("previous"), 0o600))

	path, err := NewDownloader(Config{
		URL:          serveContent(t, "k0s-binary"),
		ExpectedHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Hasher:       sha256.New(),
		DownloadDir:  dir,
		Filename:     "k0s.tmp",
	}).Download(t.Context())

	assert.ErrorContains(t, err, "hash mismatch")
	assert.Empty(t, path)

	content, err := os.ReadFile(target)
	require.NoError(t, err, "the previous download must be left alone")
	assert.Equal(t, "previous", string(content))

	// The partial download must not be left lying around either.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{"k0s.tmp"}, names)
}

func TestDownload_BadStatusLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	path, err := NewDownloader(Config{
		URL:         srv.URL,
		DownloadDir: dir,
		Filename:    "k0s.tmp",
	}).Download(t.Context())

	assert.ErrorContains(t, err, "download failed")
	assert.Empty(t, path)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a failed download must not leave anything behind")
}
