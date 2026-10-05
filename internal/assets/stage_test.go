// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"os"
	"path/filepath"
	"testing"

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
