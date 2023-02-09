/*
Copyright 2021 k0s authors

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

package dir_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopy_empty(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	srcDirname := filepath.Base(src)

	require.NoError(t, dir.Copy(src, dst), "Unable to copy empty dir")

	//rmdir will fail if the directory has anything at all
	require.NoError(t, os.Remove(path.Join(dst, srcDirname)), "Unable to remove supposedly empty dir")
}

func TestCopy_FilesAndDirs(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	srcDirname := filepath.Base(src)

	expectedDirs := []string{"dir1/", "dir2/", "dir2/dir1/", "dir2/dir2/"}
	expectedFiles := []string{"dir1/file1", "dir1/file2", "dir2/file1", "dir2/dir2/file1"}

	for _, dir := range expectedDirs {
		p := filepath.Join(src, dir)
		require.NoErrorf(t, os.Mkdir(p, 0700), "Unable to create directory %s", p)
	}

	for _, file := range expectedFiles {
		p := filepath.Join(src, file)
		require.NoError(t, os.WriteFile(p, []byte{}, 0600), "Unable to create file %s:", p)
	}

	require.NoError(t, dir.Copy(src, dst), "Unable to copy dir with contents")

	destPath := filepath.Join(dst, srcDirname)
	for _, file := range expectedFiles {
		assert.FileExists(t, filepath.Join(destPath, file), "File not copied")
	}
}
