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
	"github.com/rqlite/rqlite/store"
	"github.com/sirupsen/logrus"
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
	return "sqlite"
}

func (s *sqliteStep) Backup() (StepResult, error) {
	dbPath, err := s.getKineDBPath()
	if err != nil {
		return StepResult{}, err
	}
	kineDB, err := db.Open(dbPath)
	path := filepath.Join(s.tmpDir, kineBackup)

	logrus.Debugf("exporting kine db to %v", path)
	f, err := os.Create(path)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to create kine backup: %v", err)
	}
	// create a hot backup of the kine db
	err = kineDB.Dump(f)
	if err != nil {
		return StepResult{}, fmt.Errorf("failed to back-up kine db: %v", err)
	}
	return StepResult{filesForBackup: []string{path}}, nil
}

func (s *sqliteStep) Restore(restoreFrom string, _ string) error {
	dbConf := store.NewDBConfig(s.dataSource, false)
	db := store.New(&store.StoreConfig{
		DBConf:    dbConf,
		Dir:       s.dataDir,
		Tn:        nil,
		Logger:    nil,
		PeerStore: nil,
	})

	snapshotPath := filepath.Join(restoreFrom, kineBackup)
	snapFile, err := os.Open(snapshotPath)
	if err != nil {
		logrus.Errorf("failed to open snapshot file: %v", err)
	}
	if err := db.Restore(snapFile); err != nil {
		logrus.Errorf("failed to restore snapshot from disk: %v", err)
	}
	return nil
}
func (s *sqliteStep) getKineDBPath() (string, error) {
	u, err := url.Parse(s.dataSource)
	if err != nil {
		return "", fmt.Errorf("failed to parse Kind datasource string: %v", err)
	}
	logrus.Debugf("kinedb path: %v", u.Path)
	return u.Path, nil
}
