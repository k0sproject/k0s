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

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
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
	_, childName := path.Split(d.path)
	objectPathInArchive := path.Join(restoreFrom, childName)
	objectPathInRestored := path.Join(restoreTo, childName)
	stat, err := os.Stat(objectPathInArchive)
	if os.IsNotExist(err) {
		logrus.Debugf("Path `%s` not found in the archive, skipping...", objectPathInArchive)
		return nil
	}
	logrus.Infof("restoring from `%s` to `%s`", objectPathInArchive, restoreTo)
	if stat.IsDir() {
		return dir.Copy(objectPathInArchive, restoreTo)
	}
	return file.Copy(objectPathInArchive, objectPathInRestored)
}

// NewFileSystemStep constructor
func NewFileSystemStep(path string) FileSystemStep {
	return FileSystemStep{path: path}

}
