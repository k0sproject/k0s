package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// return true if currently running executable is older than given filepath
func ExecutableIsOlder(filepath string) bool {
	ex, err := os.Executable()
	if err != nil {
		return false
	}
	exinfo, err := os.Stat(ex)
	if err != nil {
		return false
	}
	pathinfo, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return exinfo.ModTime().Unix() < pathinfo.ModTime().Unix()
}

// Stage ...
func Stage(dataDir, name string) error {
	p := filepath.Join(dataDir, name)

	if ExecutableIsOlder(p) {
		logrus.Debug("Re-use existing file:", p)
		return nil
	}

	content, err := Asset(name)
	if err != nil {
		return err
	}
	logrus.Debug("Writing static file: ", p)
	err = os.MkdirAll(filepath.Dir(p), 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
	}
	os.Remove(p)
	if err := ioutil.WriteFile(p, content, 0600); err != nil {
		return errors.Wrapf(err, "failed to write to %s", name)
	}
	if err := os.Chmod(p, 0500); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", name)
	}

	return nil
}
