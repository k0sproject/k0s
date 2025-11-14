//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
)

const kineBackup = "kine-state-backup.db"

// sqliteDB wraps sql.DB to provide backup functionality
type sqliteDB struct {
	*sql.DB
}

func openDB(path string) (*sqliteDB, error) {
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return &sqliteDB{DB: db}, nil
}

func (db *sqliteDB) Backup(path string) error {
	_, err := db.Exec("VACUUM INTO ?", path)
	return err
}

type sqliteStep struct {
	dbPath string
	tmpDir string
}

func newSqliteStep(tmpDir string, dbPath string) *sqliteStep {
	return &sqliteStep{
		tmpDir: tmpDir,
		dbPath: dbPath,
	}
}

func (s *sqliteStep) Name() string {
	return "sqlite db path " + s.dbPath
}

func (s *sqliteStep) Backup() (StepResult, error) {
	kineDB, err := openDB(s.dbPath)
	if err != nil {
		return StepResult{}, err
	}
	defer kineDB.Close()
	path := filepath.Join(s.tmpDir, kineBackup)

	logrus.Debugf("exporting kine db to %v", path)
	_, err = os.Create(path)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to create kine backup: %w", err)
	}
	// create a hot backup of the kine db
	err = kineDB.Backup(path)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to back-up kine db: %w", err)
	}
	return StepResult{filesForBackup: []string{path}}, nil
}

func (s *sqliteStep) Restore(restoreFrom string, _ string) error {
	snapshotPath := filepath.Join(restoreFrom, kineBackup)
	if !file.Exists(snapshotPath) {
		return fmt.Errorf("sqlite snapshot not found at %s", snapshotPath)
	}

	// make sure DB dir exists. if not, create it.
	dbPathDir := filepath.Dir(s.dbPath)
	if err := dir.Init(dbPathDir, constant.KineDBDirMode); err != nil {
		return err
	}
	logrus.Infof("restoring sqlite db to `%s`", s.dbPath)
	if err := file.Copy(snapshotPath, s.dbPath); err != nil {
		logrus.Errorf("failed to restore snapshot from disk: %v", err)
	}
	return nil
}
