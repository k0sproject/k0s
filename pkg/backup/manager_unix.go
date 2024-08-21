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
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/archive"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/config/kine"
)

// Manager hold configuration for particular backup-restore process
type Manager struct {
	steps   []Backuper
	tmpDir  string
	dataDir string
}

// RunBackup backups cluster
func (bm *Manager) RunBackup(nodeSpec *v1beta1.ClusterSpec, vars *config.CfgVars, savePathDir string, out io.Writer) error {
	_, err := vars.NodeConfig()
	if err != nil {
		return err
	}

	bm.discoverSteps(vars.StartupConfigPath, nodeSpec, vars, "backup", "", out)
	defer os.RemoveAll(bm.tmpDir)
	assets := make([]string, 0, len(bm.steps))

	logrus.Info("Starting backup")
	for _, step := range bm.steps {
		logrus.Info("Backup step: ", step.Name())
		result, err := step.Backup()
		if err != nil {
			return fmt.Errorf("failed to create backup on step `%s`: %w", step.Name(), err)
		}
		assets = append(assets, result.filesForBackup...)
	}

	if savePathDir == "-" {
		return createArchive(out, assets, bm.dataDir)
	}

	backupFileName := fmt.Sprintf("k0s_backup_%s.tar.gz", timeStamp())
	if err := bm.save(backupFileName, assets); err != nil {
		return fmt.Errorf("failed to create archive `%s`: %w", backupFileName, err)
	}
	srcBackupFile := filepath.Join(bm.tmpDir, backupFileName)
	destBackupFile := filepath.Join(savePathDir, backupFileName)
	if err := file.Copy(srcBackupFile, destBackupFile); err != nil {
		return fmt.Errorf("failed to rename temporary archive: %w", err)
	}
	logrus.Infof("archive %s created successfully", destBackupFile)
	return nil
}

func (bm *Manager) discoverSteps(configFilePath string, nodeSpec *v1beta1.ClusterSpec, vars *config.CfgVars, action string, restoredConfigPath string, out io.Writer) {
	switch nodeSpec.Storage.Type {
	case v1beta1.EtcdStorageType:
		if nodeSpec.Storage.Etcd.IsExternalClusterUsed() {
			logrus.Warnf("%s is not supported for an external etcd cluster, it must be done manually", action)
		} else {
			bm.Add(newEtcdStep(bm.tmpDir, vars.CertRootDir, vars.EtcdCertDir, nodeSpec.Storage.Etcd.PeerAddress, vars.EtcdDataDir))
		}

	case v1beta1.KineStorageType:
		if backend, dsn, err := kine.SplitDataSource(nodeSpec.Storage.Kine.DataSource); err != nil {
			logrus.WithError(err).Warnf("cannot %s kine data source, it must be done manually", action)
		} else if backend != "sqlite" {
			logrus.Warnf("%s is not supported for %q kine data sources, it must be done manually", action, backend)
		} else if dbPath, err := kine.GetSQLiteFilePath(vars.DataDir, dsn); err != nil {
			logrus.WithError(err).Warnf("cannot %s SQLite database file, it must be done manually", action)
		} else {
			bm.Add(newSqliteStep(bm.tmpDir, dbPath))
		}
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
		bm.Add(NewFileSystemStep(path))
	}
	bm.Add(newConfigurationStep(configFilePath, restoredConfigPath, out))
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
		return fmt.Errorf("error creating archive file: %w", err)
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
		return fmt.Errorf("failed to copy archive file from temporary directory: %w", err)
	}
	return nil
}

// RunRestore restores cluster
func (bm *Manager) RunRestore(archivePath string, k0sVars *config.CfgVars, desiredRestoredConfigPath string, out io.Writer) error {
	var input io.Reader
	if archivePath == "-" {
		input = os.Stdin
	} else {
		i, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer i.Close()
		input = i
	}
	if err := archive.Extract(input, bm.tmpDir); err != nil {
		return fmt.Errorf("failed to unpack backup archive `%s`: %w", archivePath, err)
	}
	defer os.RemoveAll(bm.tmpDir)
	cfg, err := bm.getConfigForRestore(k0sVars)
	if err != nil {
		return fmt.Errorf("failed to parse backed-up configuration file, check the backup archive: %w", err)
	}
	bm.discoverSteps(fmt.Sprintf("%s/k0s.yaml", bm.tmpDir), cfg.Spec, k0sVars, "restore", desiredRestoredConfigPath, out)
	logrus.Info("Starting restore")

	for _, step := range bm.steps {
		logrus.Info("Restore step: ", step.Name())
		if err := step.Restore(bm.tmpDir, bm.dataDir); err != nil {
			return fmt.Errorf("failed to restore on step `%s`: %w", step.Name(), err)
		}
	}
	return nil
}

func (bm Manager) getConfigForRestore(k0sVars *config.CfgVars) (*v1beta1.ClusterConfig, error) {
	configFromBackup := path.Join(bm.tmpDir, "k0s.yaml")
	logrus.Debugf("Using k0s.yaml from: %s", configFromBackup)

	cfg, err := k0sVars.NodeConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// NewBackupManager builds new manager
func NewBackupManager() (*Manager, error) {
	tmpDir, err := os.MkdirTemp("", "k0s-backup")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
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
