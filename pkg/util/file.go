package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

// CheckDirPermissions checks the correct permissions are set for a directory
func CheckDirPermissions(path string, perm os.FileMode) error {
	dirInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	dirMode := dirInfo.Mode().Perm()
	if dirMode != perm {
		return fmt.Errorf("directory %q exist, but the permission is %#o. The expected permission is %o", path, dirMode, perm)
	}
	return nil
}

// CheckDirWriteable checks if dir is writable by writing and removing a file
// to dir. It returns nil if dir is writable.
func CheckDirWriteable(path string) error {
	f := filepath.Join(path, ".touch")
	if err := ioutil.WriteFile(f, []byte(""), 0600); err != nil {
		return err
	}
	return os.Remove(f)
}

// InitDirectory creates a path if it does not exist, and verifies its permissions, if it does
func InitDirectory(path string, perm os.FileMode) error {
	// if directory doesn't exist, this will create it
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	// Check permissions in case directory already existed
	if err := CheckDirPermissions(path, perm); err != nil {
		return err
	}

	// verify deepest directory is writeable
	if err := CheckDirWriteable(path); err != nil {
		return err
	}

	return nil
}
