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
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/stretchr/testify/require"
)

func TestGetAll(t *testing.T) {
	t.Run("empty", func(t *testing.T) {

		tmpDir := t.TempDir()
		dirs, err := dir.GetAll(tmpDir)
		require.NoError(t, err)
		require.Empty(t, dirs)
	})
	t.Run("filter dirs", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, dir.Init(filepath.Join(tmpDir, "dir1"), 0750))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1"), []byte{}, 0600), "Unable to create file %s:", "file1")
		dirs, err := dir.GetAll(tmpDir)
		require.NoError(t, err)
		require.Equal(t, []string{"dir1"}, dirs)
	})
}
