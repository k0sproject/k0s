package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Stage ...
func Stage(dataDir string) error {
	for _, name := range AssetNames() {
		content, err := Asset(name)
		if err != nil {
			return err
		}
		p := filepath.Join(dataDir, name)
		logrus.Debug("Writing static file: ", p)
		err = os.MkdirAll(filepath.Dir(p), 0700)
		if err != nil {
			return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
		}
		os.Remove(p);
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", name)
		}
		if err := os.Chmod(p, 0500); err != nil {
			return errors.Wrapf(err, "failed to chmod %s", name)
		}
	}

	return nil
}
