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

package dir

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opencontainers/selinux/go-selinux"
)

type DirOptions struct {
	path         string
	perm         os.FileMode
	seLinuxLabel string
}

// InitWithOptions prepares directory initialization with the given path.
func InitWithOptions(path string) *DirOptions {
	return &DirOptions{
		path: path,
	}
}

// WithPermissions sets the desired permissions for the directory.
func (o *DirOptions) WithPermissions(perm os.FileMode) *DirOptions {
	o.perm = perm
	return o
}

// WithSELinuxLabel sets the desired SELinux label for the directory.
// Will only be applied if SELinux is enabled.
func (o *DirOptions) WithSELinuxLabel(label string) *DirOptions {
	o.seLinuxLabel = label
	return o
}

// Apply creates the directory with the specified options.
func (o *DirOptions) Apply() error {
	if o.path == "" {
		return errors.New("init dir: path cannot be empty")
	}

	// Create directory
	if err := os.MkdirAll(o.path, o.perm); err != nil {
		return err
	}

	// Set permissions
	if err := os.Chmod(o.path, o.perm); err != nil {
		return err
	}

	// Set SELinux label if specified and enabled
	if o.seLinuxLabel != "" && selinux.GetEnabled() {
		if err := selinux.SetFileLabel(o.path, o.seLinuxLabel); err != nil {
			return err
		}
	}

	return nil
}

// IsDirectory check the given path exists and is a directory
func IsDirectory(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.Mode().IsDir()
}

// GetAll return a list of dirs in given base path
func GetAll(base string) ([]string, error) {
	var dirs []string
	if !IsDirectory(base) {
		return dirs, fmt.Errorf("%s is not a directory", base)
	}
	fileInfos, err := os.ReadDir(base)
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

// Init creates a path if it does not exist, and verifies its permissions, if it does
func Init(path string, perm os.FileMode) error {
	return InitWithOptions(path).WithPermissions(perm).Apply()
}

// PathListJoin uses the OS path list separator to join a list of strings for things like PATH=x:y:z
func PathListJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathListSeparator))
}
