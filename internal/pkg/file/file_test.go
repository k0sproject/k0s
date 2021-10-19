package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "testExist")
	if err != nil {
		t.Errorf("failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(dir)

	// test no-existing
	got := Exists(filepath.Join(dir, "/non-existing"))
	want := false
	if got != want {
		t.Errorf("test non-existing: got %t, wanted %t", got, want)
	}

	f, err := os.CreateTemp(dir, "testExist")
	if err != nil {
		t.Errorf("failed to create temp file: %v", err)
		return
	}
	defer f.Close()

	// test existing
	got = Exists(f.Name())
	want = true
	if got != want {
		t.Errorf("test existing tempfile %s: got %t, wanted %t", f.Name(), got, want)
	}

	// test what happens when we dont have permissions to the directory to file and
	// can confirm that it actually exists
	err = os.Chmod(dir, 0000)
	if err != nil {
		t.Errorf("failed to Chmod %s", dir)
	}

	got = Exists(f.Name())
	want = false
	if got != want {
		t.Errorf("test existing tempfile %s: got %t, wanted %t", f.Name(), got, want)
	}

}
