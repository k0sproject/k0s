package util

import (
	"fmt"
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

// CheckPathPermissions checks the correct permissions are for a path (file or directory)
func CheckPathPermissions(path string, perm os.FileMode) error {
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
