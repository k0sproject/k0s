package assets

import (
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExecutableIsOlder return true if currently running executable is older than given filepath
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

// BinPath searches for a binary on disk:
// - in the BinDir folder,
// - in the PATH.
// The first to be found is the one returned.
func BinPath(name string) string {
	// Look into the BinDir folder.
	path := filepath.Join(constant.BinDir, name)
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return path
	}

	// If we still haven't found the executable, look for it in the PATH.
	if path, err := exec.LookPath(name); err == nil {
		path, _ := filepath.Abs(path)
		return path
	}
	return ""
}

// Stage ...
func Stage(dataDir string, name string, filemode os.FileMode, group string) error {
	p := filepath.Join(dataDir, name)
	logrus.Infof("Staging %s", p)

	err := util.InitDirectory(filepath.Dir(p), filemode)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
	}

	/* set group owner of the directories */
	gid, err := util.GetGID(group)
	if err != nil {
		logrus.Error(err)
	}
	if gid != 0 {
		for _, path := range []string{dataDir, filepath.Dir(p)} {
			logrus.Debugf("setting group ownership for %s to %d", path, gid)
			err := os.Chown(path, -1, gid)
			if err != nil {
				return err
			}
		}
	}

	if ExecutableIsOlder(p) {
		logrus.Debug("Re-use existing file:", p)
		return nil
	}

	gzname := "bin/" + name + ".gz"
	bin, embedded := BinData[gzname]
	if !embedded {
		logrus.Debug("Skipping not embedded file:", gzname)
		return nil
	}
	logrus.Debugf("%s is at offset %d", gzname, bin.offset)

	selfexe, err := os.Executable()
	if err != nil {
		logrus.Warn(err)
		return err
	}
	infile, err := os.Open(selfexe)
	if err != nil {
		logrus.Warn("Failed to open ", os.Args[0])
		return err
	}
	defer infile.Close()

	// find location at EOF - BinDataSize + offs
	if _, err := infile.Seek(-BinDataSize+bin.offset, 2); err != nil {
		return errors.Wrapf(err, "Failed to create gzip reader for %s", name)
	}
	gz, err := gzip.NewReader(io.LimitReader(infile, bin.size))
	if err != nil {
		return errors.Wrapf(err, "Failed to create gzip reader for %s", name)
	}

	logrus.Debug("Writing static file: ", p)

	os.Remove(p)
	f, err := os.Create(p)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", p)
	}
	defer f.Close()

	err = f.Chmod(0550)
	if err != nil {
		return errors.Wrapf(err, "failed to chmod %s", p)
	}

	_, err = io.Copy(f, gz)
	if err != nil {
		return errors.Wrapf(err, "failed to write to %s", name)
	}

	return nil
}
