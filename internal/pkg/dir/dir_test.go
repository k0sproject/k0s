/*
Copyright 2022 k0s authors

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

package dir

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// CheckPermissions checks the correct permissions
func checkPermissions(t *testing.T, path string, want os.FileMode) {
	info, err := os.Stat(path)
	require.NoError(t, err, path)
	assert.Equalf(t, want, info.Mode().Perm(), "%s has unexpected permissions", path)
}

func TestInit(t *testing.T) {
	dir := t.TempDir()

	foo := filepath.Join(dir, "foo")
	require.NoError(t, Init(foo, 0700), "failed to create temp dir foo")

	checkPermissions(t, foo, 0700)

	oldUmask := unix.Umask(0027)
	t.Cleanup(func() { unix.Umask(oldUmask) })

	bar := filepath.Join(dir, "bar")
	require.NoError(t, Init(bar, 0755), "failed to create temp dir bar")
	checkPermissions(t, bar, 0755)
}

func TestCopy_empty(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	srcDirname := path.Base(src)

	require.NoError(t, Copy(src, dst), "Unable to copy empty dir")

	//rmdir will fail if the directory has anything at all
	require.NoError(t, os.Remove(path.Join(dst, srcDirname)), "Unable to remove supposedly empty dir")
}

func TestCopy_FilesAndDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	srcDirname := path.Base(src)

	expectedDirs := []string{"dir1/", "dir2/", "dir2/dir1/", "dir2/dir2/"}
	expectedFiles := []string{"dir1/file1", "dir1/file2", "dir2/file1", "dir2/dir2/file1"}

	for _, dir := range expectedDirs {
		p := path.Join(src, dir)
		require.NoErrorf(t, os.Mkdir(p, 0700), "Unable to create directory %s", p)
	}

	for _, file := range expectedFiles {
		p := path.Join(src, file)
		require.NoError(t, os.WriteFile(p, []byte{}, 0600), "Unable to create file %s:", p)
	}

	require.NoError(t, Copy(src, dst), "Unable to copy dir with contents")

	destPath := path.Join(dst, srcDirname)
	for _, file := range expectedFiles {
		assert.FileExists(t, path.Join(destPath, file), "File not copied")
	}
}
