package util

import (
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

func GetAllDirs(base string) ([]string, error) {
	var dirs []string
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
