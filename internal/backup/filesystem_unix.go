//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"fmt"
	"os"
	"path/filepath"

	cp "github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

// FileSystemStep creates backup for file system object
type FileSystemStep struct {
	path string
}

func (d FileSystemStep) Name() string {
	return fmt.Sprintf("filesystem path `%s`", d.path)
}

func (d FileSystemStep) Backup() (StepResult, error) {
	var files []string
	if err := filepath.Walk(d.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("Path `%s` does not exist, skipping...", d.path)
				return nil
			}
			return err
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return StepResult{}, err
	}
	return StepResult{filesForBackup: files}, nil
}

func (d FileSystemStep) Restore(restoreFrom, restoreTo string) error {
	childName := filepath.Base(d.path)
	objectPathInArchive := filepath.Join(restoreFrom, childName)
	err := cp.Copy(filepath.Join(restoreFrom, childName), filepath.Join(restoreTo, childName))
	if os.IsNotExist(err) {
		logrus.Debugf("Path `%s` not found in the archive, skipping...", objectPathInArchive)
		return nil
	}
	logrus.Infof("restoring from `%s` to `%s`", objectPathInArchive, restoreTo)
	return err
}

// NewFileSystemStep constructor
func NewFileSystemStep(path string) FileSystemStep {
	return FileSystemStep{path: path}

}
