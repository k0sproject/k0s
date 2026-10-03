// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTarGz(t *testing.T, entries []struct {
	name string
	dir  bool
	data string
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, e := range entries {
		if e.dir {
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name:     e.name,
				Typeflag: tar.TypeDir,
				Mode:     0750,
			}))
			continue
		}
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Typeflag: tar.TypeReg,
			Mode:     0640,
			Size:     int64(len(e.data)),
		}))
		_, err := tw.Write([]byte(e.data))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	data := buildTarGz(t, []struct {
		name string
		dir  bool
		data string
	}{
		{name: "somedir", dir: true},
		{name: "somedir/file.txt", data: "hello"},
	})

	dst := t.TempDir()
	require.NoError(t, archive.Extract(bytes.NewReader(data), dst))

	content, err := os.ReadFile(filepath.Join(dst, "somedir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))
}

func TestExtract_RejectsPathTraversal(t *testing.T) {
	data := buildTarGz(t, []struct {
		name string
		dir  bool
		data string
	}{
		{name: "../evil.txt", data: "pwned"},
	})

	dst := t.TempDir()
	err := archive.Extract(bytes.NewReader(data), dst)
	require.ErrorContains(t, err, "illegal file path")

	_, err = os.Stat(filepath.Join(filepath.Dir(dst), "evil.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestExtract_InvalidGzip(t *testing.T) {
	err := archive.Extract(bytes.NewReader([]byte("not a gzip stream")), t.TempDir())
	require.Error(t, err)
}
