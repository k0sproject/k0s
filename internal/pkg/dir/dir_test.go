package dir

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// CheckPermissions checks the correct permissions
func checkPermissions(t *testing.T, path string, want os.FileMode) {
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return
	}
	got := info.Mode().Perm()
	if got != want {
		t.Errorf("%s has permission %o. Expected is %o", path, got, want)
	}
}

func TestInit(t *testing.T) {
	dir, err := os.MkdirTemp("", "testExist")
	if err != nil {
		t.Errorf("failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(dir)

	foo := filepath.Join(dir, "foo")
	err = Init(foo, 0700)
	if err != nil {
		t.Errorf("failed to create temp dir foo: %v", err)
	}
	checkPermissions(t, foo, 0700)

	oldUmask := unix.Umask(0027)
	bar := filepath.Join(dir, "bar")
	err = Init(bar, 0755)
	if err != nil {
		t.Errorf("failed to create temp dir bar: %v", err)
	}
	checkPermissions(t, bar, 0755)

	_ = unix.Umask(oldUmask)

}
