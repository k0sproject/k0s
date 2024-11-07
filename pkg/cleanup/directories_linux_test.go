/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cleanup

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/k0sproject/k0s/internal/os/unix"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupBeneath_NonExistent(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	dir := t.TempDir()

	err := cleanupBeneath(log, filepath.Join(dir, "non-existent"))
	assert.NoError(t, err)
	assert.DirExists(t, dir)
}

func TestCleanupBeneath_Symlinks(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	unrelatedDir := t.TempDir()
	cleanDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(unrelatedDir, "regular_file"), nil, 0644))
	require.NoError(t, os.Mkdir(filepath.Join(unrelatedDir, "regular_dir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(unrelatedDir, "regular_dir", "some_file"), nil, 0644))

	require.NoError(t, os.WriteFile(filepath.Join(cleanDir, "regular_file"), nil, 0644))
	require.NoError(t, os.Mkdir(filepath.Join(cleanDir, "regular_dir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(cleanDir, "regular_dir", "some_file"), nil, 0644))

	require.NoError(t, os.Symlink(filepath.Join(unrelatedDir, "regular_file"), filepath.Join(cleanDir, "symlinked_file")))
	require.NoError(t, os.Symlink(filepath.Join(unrelatedDir, "regular_dir"), filepath.Join(cleanDir, "symlinked_dir")))

	err := cleanupBeneath(log, filepath.Join(cleanDir))
	assert.NoError(t, err)
	assert.NoDirExists(t, cleanDir)
	assert.FileExists(t, filepath.Join(unrelatedDir, "regular_file"))
	assert.DirExists(t, filepath.Join(unrelatedDir, "regular_dir"))
}

func TestGetPathMountStatus(t *testing.T) {
	parent, err := unix.OpenDir(t.TempDir(), 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, parent.Close()) })

	t.Run("file", func(t *testing.T) {
		file, err := parent.Open("file", syscall.O_CREAT, 0644)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, file.Close()) })

		status, err := getPathMountStatus(parent, file, filepath.Join(parent.Name(), file.Name()))
		if assert.NoError(t, err) {
			assert.Equal(t, pathMountStatusUnknown, status)
		}
	})

	t.Run("dir", func(t *testing.T) {
		require.NoError(t, parent.Mkdir("dir", 0755))
		dir, err := parent.OpenDir("dir", 0)
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, dir.Close()) })

		status, err := getPathMountStatus(parent, dir, filepath.Join(parent.Name(), dir.Name()))
		if assert.NoError(t, err) {
			assert.Equal(t, pathMountStatusUnknown, status)
		}
	})
}

func TestMountInfoListsMountPoint(t *testing.T) {
	for _, path := range []string{
		`/`,
		`/dev`,
		`/sys/fs/bpf`,
		`/mnt/path with spaces`,
		`/mnt/path\with\backslashes`,
	} {
		ok, err := mountInfoListsMountPoint("testdata/mountinfo", path)
		if assert.NoError(t, err, "For %s", path) {
			assert.True(t, ok, "For %s", path)
		}
	}

	for _, path := range []string{
		``,
		`/de`,
		`/dev/`,
		`/mnt/path with space`,
		`/mnt/path with spaces/`,
		`/mnt/path\040with\040spaces`,
		`/mnt/path\with\backslash`,
		`/mnt/path\with\backslashes/`,
	} {
		ok, err := mountInfoListsMountPoint("testdata/mountinfo", path)
		if assert.NoError(t, err, "For %s", path) {
			assert.False(t, ok, "For %s", path)
		}
	}
}
