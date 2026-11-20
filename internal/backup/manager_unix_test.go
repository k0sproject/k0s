//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/internal/backup"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_RunBackup(t *testing.T) {
	t.Run("names the backup file after the current UTC time", func(t *testing.T) {
		const now, expectedFileName = 1794557785826, "k0s_backup_2026-11-13T08_16_25_826Z.tar.gz"
		configPath := filepath.Join(t.TempDir(), "k0s.yaml")
		require.NoError(t, os.WriteFile(configPath, nil, 0644))
		vars := &config.CfgVars{DataDir: t.TempDir(), StartupConfigPath: configPath}
		nodeSpec := &v1beta1.ClusterSpec{Storage: &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: &v1beta1.KineConfig{DataSource: "made-up://"},
		}}
		saveDir := t.TempDir()

		synctest.Test(t, func(t *testing.T) {
			time.Sleep(time.Until(time.UnixMilli(now)))
			// Use a local time zone that's off UTC, so that UTC normalization can be tested.
			local := time.Local
			t.Cleanup(func() { time.Local = local })
			time.Local = time.FixedZone(t.Name(), 1337)

			underTest, err := backup.NewBackupManager()
			require.NoError(t, err)
			require.NoError(t, underTest.RunBackup(t.Context(), nodeSpec, vars, saveDir, nil))
		})

		entries, err := os.ReadDir(saveDir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, expectedFileName, entries[0].Name())
	})
}
