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
