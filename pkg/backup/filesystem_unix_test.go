//go:build unix

/*
Copyright 2023 k0s authors

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

package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSystemStepBackup(t *testing.T) {
	tempDir := t.TempDir()

	file1Path := filepath.Join(tempDir, "file1")
	require.NoError(t, os.WriteFile(file1Path, []byte("file1 data"), 0644))

	dirPath := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.MkdirAll(dirPath, 0755))

	file2Path := filepath.Join(tempDir, "subdir/file2")
	require.NoError(t, os.WriteFile(file2Path, []byte("file2 data"), 0644))

	t.Run("tree", func(t *testing.T) {
		step := NewFileSystemStep(tempDir)

		result, err := step.Backup()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{tempDir, file1Path, file2Path, dirPath}, result.filesForBackup)
	})

	t.Run("file", func(t *testing.T) {
		step := NewFileSystemStep(file2Path)

		result, err := step.Backup()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{file2Path}, result.filesForBackup)
	})

	t.Run("non-existing", func(t *testing.T) {
		nonExistingPath := filepath.Join(tempDir, "no-existing")
		step := NewFileSystemStep(nonExistingPath)

		result, err := step.Backup()
		require.NoError(t, err)
		require.ElementsMatch(t, []string{}, result.filesForBackup)
	})
}

func TestFileSystemStepRestore(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()

		file1Path := filepath.Join(src, "file1")
		require.NoError(t, os.WriteFile(file1Path, []byte("file1 data"), 0644))

		step := NewFileSystemStep(file1Path)
		require.NoError(t, step.Restore(src, dst))
		require.FileExists(t, filepath.Join(dst, filepath.Base(file1Path)))
	})

	t.Run("dir", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		dirPath := filepath.Join(src, "subdir")
		require.NoError(t, os.MkdirAll(dirPath, 0755))

		file2Path := filepath.Join(src, "subdir/file2")
		require.NoError(t, os.WriteFile(file2Path, []byte("file2 data"), 0644))

		step := NewFileSystemStep(dirPath)
		require.NoError(t, step.Restore(src, dst))
		require.FileExists(t, filepath.Join(dst, "subdir/file2"))
	})

	t.Run("non-existing", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		nonExistingPath := filepath.Join(src, "no-existing")
		step := NewFileSystemStep(nonExistingPath)
		require.NoError(t, step.Restore(src, dst))
	})

	t.Run("empty", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()
		step := NewFileSystemStep(src)
		require.NoError(t, step.Restore(src, dst), "Unable to copy empty dir")

		//rmdir will fail if the directory has anything at all
		require.NoError(t, os.Remove(dst), "Unable to remove supposedly empty dir")
	})

	t.Run("existing target dir", func(t *testing.T) {
		src := t.TempDir()
		dst := t.TempDir()

		require.NoError(t, os.Mkdir(filepath.Join(dst, "dir1"), 0700))

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

		step1 := NewFileSystemStep(filepath.Join(src, "dir1"))
		require.NoError(t, step1.Restore(src, dst), "Unable to copy dir with contents")

		step2 := NewFileSystemStep(filepath.Join(src, "dir2"))
		require.NoError(t, step2.Restore(src, dst), "Unable to copy dir with contents")

		for _, file := range expectedFiles {
			require.FileExists(t, filepath.Join(dst, file))
		}
	})
}
