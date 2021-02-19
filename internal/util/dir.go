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
package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
)

// IsDirectory check the given path exists and is a directory
func IsDirectory(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.Mode().IsDir()
}

// GetAllDirs return a list of dirs in given base path
func GetAllDirs(base string) ([]string, error) {
	var dirs []string
	if !IsDirectory(base) {
		return dirs, fmt.Errorf("%s is not a directory", base)
	}
	fileInfos, err := ioutil.ReadDir(base)
	if err != nil {
		return dirs, err
	}

	for _, f := range fileInfos {
		if f.IsDir() {
			dirs = append(dirs, f.Name())
		}
	}
	return dirs, nil
}

// InitDirectory creates a path if it does not exist, and verifies its permissions, if it does
func InitDirectory(path string, perm os.FileMode) error {
	// if directory doesn't exist, this will create it
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	// Check permissions in case directory already existed
	if err := CheckPathPermissions(path, perm); err != nil {
		return err
	}

	return nil
}

// HomeDir fetches the running user's home directory, regardless of Sudo
func HomeDir() (string, error) {
	var runUser string

	if os.Getenv("SUDO_USER") != "" {
		runUser = os.Getenv("SUDO_USER")
	} else {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("cannot resolve user")
		}
		runUser = usr.Name
	}

	usr, err := user.Lookup(runUser)
	if err != nil {
		return "", fmt.Errorf("cannot detect user's home directory: %v", err)
	}
	return usr.HomeDir, nil
}
