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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/archive"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

// Manager hold configuration for particular backup-restore process
type Manager struct {
	steps   []Backuper
	tmpDir  string
	dataDir string
}

// RunBackup backups cluster
func (bm *Manager) RunBackup(cfgPath string, nodeSpec *v1beta1.ClusterSpec, vars constant.CfgVars, savePathDir string) error {
	bm.discoverSteps(cfgPath, nodeSpec, vars, "backup", "")
	defer os.RemoveAll(bm.tmpDir)
	assets := make([]string, 0, len(bm.steps))

	logrus.Info("Starting backup")
	for _, step := range bm.steps {
		logrus.Info("Backup step: ", step.Name())
		result, err := step.Backup()
		if err != nil {
			return fmt.Errorf("failed to create backup on step `%s`: %v", step.Name(), err)
		}
		assets = append(assets, result.filesForBackup...)
	}
	backupFileName := fmt.Sprintf("k0s_backup_%s.tar.gz", timeStamp())
	if err := bm.save(backupFileName, assets); err != nil {
		return fmt.Errorf("failed to create archive `%s`: %v", backupFileName, err)
	}
	srcBackupFile := filepath.Join(bm.tmpDir, backupFileName)
	destBackupFile := filepath.Join(savePathDir, backupFileName)
	if err := file.Copy(srcBackupFile, destBackupFile); err != nil {
		return fmt.Errorf("failed to rename temporary archive: %v", err)
	}
	logrus.Infof("archive %s created successfully", destBackupFile)
	return nil
}

func (bm *Manager) discoverSteps(configFilePath string, nodeSpec *v1beta1.ClusterSpec, vars constant.CfgVars, action string, restoredConfigPath string) {
	if nodeSpec.Storage.Type == v1beta1.EtcdStorageType {
		bm.Add(newEtcdStep(bm.tmpDir, vars.CertRootDir, vars.EtcdCertDir, nodeSpec.Storage.Etcd.PeerAddress, vars.EtcdDataDir))
	} else if nodeSpec.Storage.Type == v1beta1.KineStorageType && strings.HasPrefix(nodeSpec.Storage.Kine.DataSource, "sqlite://") {
		bm.Add(newSqliteStep(bm.tmpDir, nodeSpec.Storage.Kine.DataSource, vars.DataDir))
	} else {
		logrus.Warnf("only etcd and sqlite %s is supported. Other storage backends must be backed-up/restored manually.", action)
	}
	bm.dataDir = vars.DataDir
	for _, path := range []string{
		vars.CertRootDir,
		vars.ManifestsDir,
		vars.OCIBundleDir,
		vars.HelmHome,
		vars.HelmRepositoryConfig,
	} {
		if action == "backup" {
			logrus.Infof("adding `%s` path to the backup archive", path)
		}
		bm.Add(NewFilesystemStep(path))
	}
	bm.Add(newConfigurationStep(configFilePath, restoredConfigPath))
}

// Add adds backup step
func (bm *Manager) Add(step Backuper) {
	if bm.steps == nil {
		bm.steps = []Backuper{step}
		return
	}
	bm.steps = append(bm.steps, step)
}

func (bm Manager) save(backupFileName string, assets []string) error {
	archiveFile := filepath.Join(bm.tmpDir, backupFileName)
	logrus.Debugf("creating temporary archive file: %v", archiveFile)
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

	destinationFile := filepath.Join(bm.tmpDir, backupFileName)
	err = file.Copy(archiveFile, destinationFile)
	if err != nil {
		return fmt.Errorf("failed to copy archive file from temporary directory: %v", err)
	}
	return nil
}

// RunRestore restores cluster
func (bm *Manager) RunRestore(archivePath string, k0sVars constant.CfgVars, desiredRestoredConfigPath string) error {
	if err := archive.Extract(archivePath, bm.tmpDir); err != nil {
		return fmt.Errorf("failed to unpack backup archive `%s`: %v", archivePath, err)
	}
	defer os.RemoveAll(bm.tmpDir)
	cfg, err := bm.getConfigForRestore(k0sVars)
	if err != nil {
		return fmt.Errorf("failed to parse backed-up configuration file, check the backup archive: %v", err)
	}
	bm.discoverSteps(fmt.Sprintf("%s/k0s.yaml", bm.tmpDir), cfg.Spec, k0sVars, "restore", desiredRestoredConfigPath)
	logrus.Info("Starting restore")

	for _, step := range bm.steps {
		logrus.Info("Restore step: ", step.Name())
		if err := step.Restore(bm.tmpDir, bm.dataDir); err != nil {
			return fmt.Errorf("failed to restore on step `%s`: %v", step.Name(), err)
		}
	}
	return nil
}

func (bm Manager) getConfigForRestore(k0sVars constant.CfgVars) (*v1beta1.ClusterConfig, error) {
	configFromBackup := path.Join(bm.tmpDir, "k0s.yaml")
	_, err := os.Stat(configFromBackup)
	if os.IsNotExist(err) {
		return v1beta1.DefaultClusterConfig(bm.dataDir), nil
	}
	logrus.Infof("Using k0s.yaml from: %s", configFromBackup)

	loadingRules := config.ClientConfigLoadingRules{RuntimeConfigPath: configFromBackup}
	cfg, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewBackupManager builds new manager
func NewBackupManager() (*Manager, error) {
	tmpDir, err := os.MkdirTemp("", "k0s-backup")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %v", err)
	}

	bm := &Manager{
		tmpDir: tmpDir,
	}

	return bm, nil
}

// Backuper defines interface for backup-restore step
type Backuper interface {
	Name() string
	Backup() (StepResult, error)
	Restore(from, to string) error
}

// StepResult backup result for the particular step
type StepResult struct {
	filesForBackup []string
}
