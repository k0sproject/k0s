//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/archive"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archiveEntries runs step.Backup and returns the entry names the resulting
// archive carries, the way RunBackup assembles it.
func archiveEntries(t *testing.T, step Backuper, dataDir string) []string {
	t.Helper()

	result, err := step.Backup()
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, createArchive(&buf, result.filesForBackup, dataDir))

	extracted := t.TempDir()
	require.NoError(t, archive.Extract(&buf, extracted))
	entries, err := os.ReadDir(extracted)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestConfigurationStepBackup(t *testing.T) {
	t.Run("stores_the_config_as_k0s_yaml", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "k0s.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("spec: {}\n"), 0644))

		step := newConfigurationStep(t.TempDir(), cfgPath, "", nil)
		assert.Equal(t, []string{"k0s.yaml"}, archiveEntries(t, step, t.TempDir()))
	})

	t.Run("stores_a_custom_config_file_name_as_k0s_yaml", func(t *testing.T) {
		// Restore only ever looks for k0s.yaml, so the name the cluster was
		// started with must not leak into the archive.
		cfgPath := filepath.Join(t.TempDir(), "my-cluster.yaml")
		require.NoError(t, os.WriteFile(cfgPath, []byte("spec: {}\n"), 0644))

		step := newConfigurationStep(t.TempDir(), cfgPath, "", nil)
		assert.Equal(t, []string{"k0s.yaml"}, archiveEntries(t, step, t.TempDir()))
	})

	t.Run("preserves_the_config_contents", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "custom.yaml")
		content := "spec:\n  storage:\n    type: kine\n"
		require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0644))

		tmpDir := t.TempDir()
		step := newConfigurationStep(tmpDir, cfgPath, "", nil)
		result, err := step.Backup()
		require.NoError(t, err)
		require.Len(t, result.filesForBackup, 1)

		staged, err := os.ReadFile(result.filesForBackup[0])
		require.NoError(t, err)
		assert.Equal(t, content, string(staged))
	})

	t.Run("skips_a_missing_config", func(t *testing.T) {
		// k0s runs happily without /etc/k0s/k0s.yaml, so a backup of such a
		// cluster must succeed rather than abort.
		cfgPath := filepath.Join(t.TempDir(), "k0s.yaml") // never created

		step := newConfigurationStep(t.TempDir(), cfgPath, "", nil)
		result, err := step.Backup()
		require.NoError(t, err)
		assert.Empty(t, result.filesForBackup)
	})
}

func TestGetConfigForRestore(t *testing.T) {
	t.Run("reads_k0s_yaml_from_the_archive", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "k0s.yaml"),
			[]byte("spec:\n  storage:\n    type: kine\n"), 0644))

		cfg, err := Manager{tmpDir: tmpDir}.getConfigForRestore()
		require.NoError(t, err)
		assert.Equal(t, "kine", string(cfg.Spec.Storage.Type))
	})

	t.Run("falls_back_to_defaults_when_absent", func(t *testing.T) {
		// Archives taken from a cluster that ran without a config file carry
		// no k0s.yaml; restoring them must still work.
		cfg, err := Manager{tmpDir: t.TempDir()}.getConfigForRestore()
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "etcd", string(cfg.Spec.Storage.Type))
	})
}
