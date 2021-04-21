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
	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Manager hold configuration for particular backup-restore process
type Manager struct {
	steps            []Backuper
	backupWorkingDir string
	dataDir          string
}

// RunBackup backups cluster
func (bm Manager) RunBackup(backupsDirectory string) error {
	defer os.RemoveAll(bm.backupWorkingDir)
	assets := make([]string, 0, len(bm.steps))

	logrus.Info("Starting backup")
	for _, step := range bm.steps {
		logrus.Info("Backup step: ", step.Name())
		result, err := step.Backup(bm.backupWorkingDir)
		if err != nil {
			return fmt.Errorf("failed to create backup on step `%s`: %v", step.Name(), err)
		}
		assets = append(assets, result.filesForBackup...)
	}
	backupFileName := fmt.Sprintf("k0s_backup_%s.tar.gz", timeStamp())
	return bm.save(backupFileName, assets, backupsDirectory)
}

func (bm Manager) save(backupFileName string, assets []string, backupsDirectory string) error {
	archiveFile := filepath.Join(bm.backupWorkingDir, backupFileName)
	logrus.Infof("creating temporary archive file: %v", archiveFile)
	out, err := os.Create(archiveFile)
	if err != nil {
		return fmt.Errorf("error creating archive file: %v", err)
	}
	defer out.Close()
	// Create the archive and write the output to the "out" Writer
	err = createArchive(out, assets, bm.dataDir)
	if err != nil {
		logrus.Fatalf("error creating archive: %v", err)
	}

	destinationFile := filepath.Join(backupsDirectory, backupFileName)
	err = util.FileCopy(archiveFile, destinationFile)
	if err != nil {
		return fmt.Errorf("failed to copy archive file from temporary directory: %v", err)
	}
	logrus.Infof("archive %s created successfully", destinationFile)
	return nil
}

// RunRestore restores cluster
func (bm Manager) RunRestore(archivePath string) error {
	if err := util.ExtractArchive(archivePath, bm.dataDir); err != nil {
		return err
	}
	for _, step := range bm.steps {
		if err := step.Restore(bm.dataDir); err != nil {
			return fmt.Errorf("failed to restore on step `%s`: %v", step.Name(), err)
		}
	}
	return nil
}

// NewBackupManager builds new manager
func NewBackupManager(clusterSpec *v1beta1.ClusterSpec, vars constant.CfgVars) (*Manager, error) {
	var steps []Backuper

	if clusterSpec.Storage.Type != v1beta1.EtcdStorageType {
		logrus.Warnf("non-etcd data storage backup not supported. You must take the database backup manually")
	} else {
		steps = append(steps, newEtcdStep(vars.CertRootDir, vars.EtcdCertDir, clusterSpec.Storage.Etcd.PeerAddress, vars.EtcdDataDir))
	}

	steps = append(steps, newCertsStep(vars.CertRootDir))

	tmpDir, err := ioutil.TempDir("", "k0s-backup")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %v", err)
	}

	return &Manager{
		steps:            steps,
		backupWorkingDir: tmpDir,
		dataDir:          vars.DataDir,
	}, nil
}

// Backuper defines interface for backup-restore step
type Backuper interface {
	Name() string
	Backup(workingDir string) (StepResult, error)
	Restore(restoreTo string) error
}

// StepResult backup result for the particular step
type StepResult struct {
	filesForBackup []string
}
