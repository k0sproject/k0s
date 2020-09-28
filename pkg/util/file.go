package util

import (
	"fmt"
	"io"
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

// Exist checks if a file or directory exists
func Exist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
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
		return fmt.Errorf("directory %q exist, but the permission is %q. The expected permission is %q", path, dirInfo.Mode(), perm)
	}
	return nil
}

// IsDirWriteable checks if dir is writable by writing and removing a file
// to dir. It returns nil if dir is writable.
func IsDirWriteable(path string) error {
	f := filepath.Join(path, ".touch")
	if err := ioutil.WriteFile(f, []byte(""), 0600); err != nil {
		return err
	}
	return os.Remove(f)
}

// IsDirEmpty checks if a dir is empty
func IsDirEmpty(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// read in ONLY one file
	_, err = f.Readdir(1)

	// if the file is EOF, the dir is empty
	if err == io.EOF {
		return nil
	}
	return err
}

// InitDirectory verifies:
// 	- if a directory path already exists. if it does, it verifies its permissions
// 	- if it does not, it continues to create the path, and:
//	- check that its writable, and
//	- check that it does not include other files
func InitDirectory(path string, perm os.FileMode) error {
	// if directory already exists, check correct permissions
	if Exist(path) {
		if err := CheckDirPermissions(path, perm); err != nil {
			return err
		}
	} else {
		// Directory doesn't exist, run os.MkdirAll
		if err := os.MkdirAll(path, perm); err != nil {
			return err
		}
		// verify deepest directory is writeable
		if err := IsDirWriteable(path); err != nil {
			return err
		}
		if err := IsDirEmpty(path); err != nil {
			return fmt.Errorf("directory %q is not empty", path)
		}
	}
	return nil
}
