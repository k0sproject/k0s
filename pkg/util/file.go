package util

import (
	"fmt"
	"io/ioutil"
	"os"
)

// FileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func IsDirectory(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.Mode().IsDir()
}

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
