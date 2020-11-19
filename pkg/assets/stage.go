/*
Copyright 2020 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package assets

import (
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
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
func BinPath(name string, binDir string) string {
	// Look into the BinDir folder.
	path := filepath.Join(binDir, name)
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return path
	}

	// If we still haven't found the executable, look for it in the PATH.
	if path, err := exec.LookPath(name); err == nil {
		path, _ := filepath.Abs(path)
		return path
	}
	return name
}

// Stage ...
func Stage(dataDir string, name string, filemode os.FileMode) error {
	p := filepath.Join(dataDir, name)
	logrus.Infof("Staging %s", p)

	err := util.InitDirectory(filepath.Dir(p), filemode)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
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
		return errors.Wrapf(err, "Failed to find embedded file position for %s", name)
	}
	gz, err := gzip.NewReader(io.LimitReader(infile, bin.size))
	if err != nil {
		return errors.Wrapf(err, "Failed to create gzip reader for %s", name)
	}

	logrus.Debug("Writing static file: ", p)


	if err := copyTo(p, gz); err != nil {
		return err
	}
	if err := os.Chmod(p, 0550); err != nil {
		return errors.Wrapf(err, "Failed to chmod %s", name)
	}
	return nil
}


func copyTo(p string, gz io.Reader) error {
	os.Remove(p)
	f, err := os.Create(p)
	defer f.Close()
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", p)
	}
	_, err = io.Copy(f, gz)
	if err != nil {
		return errors.Wrapf(err, "failed to write to %s", p)
	}
	return nil
}