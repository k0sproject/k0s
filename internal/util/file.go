/*
Copyright 2020 Mirantis, Inc.

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
package util

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"os/exec"
)

// FileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// CheckPathPermissions checks the correct permissions are for a path (file or directory)
func CheckPathPermissions(path string, perm os.FileMode) error {
	dirInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	dirMode := dirInfo.Mode().Perm()
	if dirMode != perm {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("directory %q exist, but the permission is %#o. The expected permission is %o", path, dirMode, perm)
		}
		logrus.Warnf("directory %q exist, but the permission is %#o. The expected permission is %o", path, dirMode, perm)
		return nil
	}
	return nil
}

// Find the path for a given file (similar to `which`)
func GetExecPath(fileName string) (*string, error) {
	path, err := exec.LookPath(fileName)
	if err != nil {
		return nil, err
	}

	return &path, nil
}
