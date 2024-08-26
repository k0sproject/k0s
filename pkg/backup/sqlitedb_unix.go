//go:build unix

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

package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rqlite/rqlite/db"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"
)

const kineBackup = "kine-state-backup.db"

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
	return fmt.Sprintf("sqlite db path %s", s.dbPath)
}

func (s *sqliteStep) Backup() (StepResult, error) {
	kineDB, err := db.Open(s.dbPath)
	if err != nil {
		return StepResult{}, err
	}
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
