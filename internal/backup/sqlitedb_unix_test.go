//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createSqliteSnapshot(t *testing.T, restoreFrom string) {
	t.Helper()

	srcPath := filepath.Join(t.TempDir(), "src.db")
	db, err := sql.Open("sqlite", srcPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO kv VALUES ('key', 'from backup')")
	require.NoError(t, err)
	_, err = db.Exec("VACUUM INTO ?", filepath.Join(restoreFrom, kineBackup))
	require.NoError(t, err)
}

func TestSqliteStepRestoreRemovesCompanionFiles(t *testing.T) {
	restoreFrom := t.TempDir()
	createSqliteSnapshot(t, restoreFrom)

	dbPath := filepath.Join(t.TempDir(), "db", "state.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0750))
	require.NoError(t, os.WriteFile(dbPath, []byte("old database"), 0600))
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		require.NoError(t, os.WriteFile(dbPath+suffix, []byte("stale "+suffix), 0600))
	}

	require.NoError(t, newSqliteStep(t.TempDir(), dbPath).Restore(restoreFrom, ""))

	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		assert.NoFileExists(t, dbPath+suffix)
	}

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var integrity string
	require.NoError(t, db.QueryRow("PRAGMA integrity_check").Scan(&integrity))
	assert.Equal(t, "ok", integrity)

	var value string
	require.NoError(t, db.QueryRow("SELECT v FROM kv WHERE k = 'key'").Scan(&value))
	assert.Equal(t, "from backup", value)
}

func TestSqliteStepRestoreWithoutCompanionFiles(t *testing.T) {
	restoreFrom := t.TempDir()
	createSqliteSnapshot(t, restoreFrom)

	dbPath := filepath.Join(t.TempDir(), "db", "state.db")

	require.NoError(t, newSqliteStep(t.TempDir(), dbPath).Restore(restoreFrom, ""))
	assert.FileExists(t, dbPath)
}

func TestSqliteStepRestoreFailsIfCleanupFails(t *testing.T) {
	restoreFrom := t.TempDir()
	createSqliteSnapshot(t, restoreFrom)

	dbPath := filepath.Join(t.TempDir(), "state.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("old database"), 0600))
	// A non-empty directory can't be removed with os.Remove.
	require.NoError(t, os.MkdirAll(filepath.Join(dbPath+"-wal", "child"), 0750))

	err := newSqliteStep(t.TempDir(), dbPath).Restore(restoreFrom, "")
	require.ErrorContains(t, err, "failed to remove stale sqlite file")

	data, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, "old database", string(data), "the database must not be replaced if cleanup fails")
}
