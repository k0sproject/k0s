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

package file

import (
	"fmt"
	"os"
	"runtime"

	"github.com/k0sproject/k0s/internal/pkg/users"

	"github.com/sirupsen/logrus"
)

// Exists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func Exists(fileName string) bool {
	info, err := os.Stat(fileName)
	if err != nil {
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

// Chown changes file/dir mode
func Chown(file, owner string, permissions os.FileMode) error {
	// Chown the file properly for the owner
	uid, _ := users.GetUID(owner)
	err := os.Chown(file, uid, -1)
	if err != nil && os.Geteuid() == 0 {
		return err
	}
	err = os.Chmod(file, permissions)
	if err != nil && os.Geteuid() == 0 {
		return err
	}
	return nil
}

// Copy copies file from src to dst
func Copy(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("error reading source file (%v): %v", src, err)
	}

	err = os.WriteFile(dst, input, sourceFileStat.Mode())
	if err != nil {
		return fmt.Errorf("error writing destination file (%v): %v", dst, err)
	}
	return nil
}

func WriteTmpFile(data string, prefix string) (path string, err error) {
	tmpFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", fmt.Errorf("cannot create temporary file: %v", err)
	}

	text := []byte(data)
	if _, err = tmpFile.Write(text); err != nil {
		return "", fmt.Errorf("failed to write to temporary file: %v", err)
	}

	return tmpFile.Name(), nil
}
