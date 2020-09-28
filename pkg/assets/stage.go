package assets

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

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

// StagedBinPath returns the path of the staged bin or the name without path if it does not exist
func StagedBinPath(dataDir, name string) string {
	p := filepath.Join(dataDir, "bin", name)
	if util.FileExists(p) {
		return p
	}
	return name
}

// Stage ...
func Stage(dataDir, name, group string) error {
	p := filepath.Join(dataDir, name)
	logrus.Infof("Staging %s", name)

	err := util.InitDirectory(filepath.Dir(p), 0750)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
	}

	/* set group owner of the directories */
	gid, _ := util.GetGID(group)
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

	gzname := name + ".gz"
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
	infile.Seek(-BinDataSize+bin.offset, 2)
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
