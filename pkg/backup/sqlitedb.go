//go:build !windows
// +build !windows

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
	"net/url"
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
	dataSource string
	tmpDir     string
	dataDir    string
}

func newSqliteStep(tmpDir string, dataSource string, dataDir string) *sqliteStep {
	return &sqliteStep{
		tmpDir:     tmpDir,
		dataSource: dataSource,
		dataDir:    dataDir,
	}
}

func (s *sqliteStep) Name() string {
	dbPath, _ := s.getKineDBPath()
	return fmt.Sprintf("sqlite db path %s", dbPath)
}

func (s *sqliteStep) Backup() (StepResult, error) {
	dbPath, err := s.getKineDBPath()
	if err != nil {
		return StepResult{}, err
	}
	kineDB, err := db.Open(dbPath)
	if err != nil {
		return StepResult{}, err
	}
	path := filepath.Join(s.tmpDir, kineBackup)

	logrus.Debugf("exporting kine db to %v", path)
	_, err = os.Create(path)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to create kine backup: %v", err)
	}
	// create a hot backup of the kine db
	err = kineDB.Backup(path)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to back-up kine db: %v", err)
	}
	return StepResult{filesForBackup: []string{path}}, nil
}

func (s *sqliteStep) Restore(restoreFrom string, _ string) error {
	snapshotPath := filepath.Join(restoreFrom, kineBackup)
	if !file.Exists(snapshotPath) {
		return fmt.Errorf("sqlite snapshot not found at %s", snapshotPath)
	}
	dbPath, err := s.getKineDBPath()
	if err != nil {
		return err
	}

	// make sure DB dir exists. if not, create it.
	dbPathDir := filepath.Dir(dbPath)
	if err = dir.Init(dbPathDir, constant.KineDBDirMode); err != nil {
		return err
	}
	logrus.Infof("restoring sqlite db to `%s`", dbPath)
	if err := file.Copy(snapshotPath, dbPath); err != nil {
		logrus.Errorf("failed to restore snapshot from disk: %v", err)
	}
	return nil
}

func (s *sqliteStep) getKineDBPath() (string, error) {
	u, err := url.Parse(s.dataSource)
	if err != nil {
		return "", fmt.Errorf("failed to parse Kind datasource string: %v", err)
	}
	return u.Path, nil
}
