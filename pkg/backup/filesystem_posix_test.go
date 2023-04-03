//go:build !windows
// +build !windows

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
