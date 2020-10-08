package util

import (
	"fmt"
	"io/ioutil"
	"os"
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
