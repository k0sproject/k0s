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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
)

type Config struct {
	k0sVars     constant.CfgVars
	storageSpec *v1beta1.StorageSpec
	savePath    string
	savedAssets []string
	tmpDir      string
}

const (
	etcdBackup = "etcd-snapshot.db"
	// kineBackup = "kine-state-backup.db"
)

func NewBackupConfig(k0sVars constant.CfgVars, storageSpec *v1beta1.StorageSpec, savePath string) *Config {
	return &Config{
		k0sVars:     k0sVars,
		storageSpec: storageSpec,
		savePath:    savePath,
	}
}

func (c *Config) RunBackup() error {
	backupFileName := fmt.Sprintf("k0s_backup_%s.tar.gz", timeStamp())

	logrus.Info("starting backup")
	tmpDir, err := ioutil.TempDir("", "k0s-backup")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	c.tmpDir = tmpDir
	archiveFile := filepath.Join(tmpDir, backupFileName)
	logrus.Infof("creating temporary archive file: %v", archiveFile)
	out, err := os.Create(archiveFile)
	if err != nil {
		return fmt.Errorf("error creating archive file: %v", err)
	}
	defer out.Close()

	if c.storageSpec.Type == v1beta1.KineStorageType {
		// Kine backup not supported, yet
		logrus.Warnf("non-etcd data storage backup not supported. You must take the database backup manually")
	} else {
		// take Etcd snapshot
		err := c.saveEtcdSnapshot()
		if err != nil {
			return fmt.Errorf("failed to create etcd snapshot: %v", err)
		}
	}
	// back-up PKI Dir contents
	err = c.saveCerts()
	if err != nil {
		return fmt.Errorf("failed to save certificates: %v", err)
	}

	// Create the archive and write the output to the "out" Writer
	err = createArchive(out, c.savedAssets, c.k0sVars.DataDir)
	if err != nil {
		logrus.Fatalf("error creating archive: %v", err)
	}

	destinationFile := filepath.Join(c.savePath, backupFileName)
	err = util.FileCopy(archiveFile, destinationFile)
	if err != nil {
		return fmt.Errorf("failed to copy archive file from temporary directory: %v", err)
	}
	logrus.Infof("archive %s created successfully", destinationFile)
	return nil
}

func (c *Config) saveCerts() error {
	err := filepath.Walk(c.k0sVars.CertRootDir, func(path string, info os.FileInfo, err error) error {
		c.savedAssets = append(c.savedAssets, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to list certificates in %v: %v", c.k0sVars.CertRootDir, err)
	}
	return nil
}
