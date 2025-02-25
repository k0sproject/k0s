/*
Copyright 2020 k0s authors

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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// EmbeddedBinaryNeedsUpdate returns true if the provided embedded binary file should
// be updated. This determination is based on the modification times and file sizes of both
// the provided executable and the embedded executable. It is expected that the embedded binary
// modification times should match the main `k0s` executable.
func EmbeddedBinaryNeedsUpdate(exinfo os.FileInfo, embeddedBinaryPath string, size int64) bool {
	if pathinfo, err := os.Stat(embeddedBinaryPath); err == nil {
		return !exinfo.ModTime().Equal(pathinfo.ModTime()) || pathinfo.Size() != size
	}

	// If the stat fails, the file is either missing or permissions are missing
	// to read this -- let above know that an update should be attempted.

	return true
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
func Stage(dataDir string, name string) error {
	p := filepath.Join(dataDir, name)
	logrus.Infof("Staging '%s'", p)

	selfexe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine current executable: %w", err)
	}

	exinfo, err := os.Stat(selfexe)
	if err != nil {
		return fmt.Errorf("unable to stat '%s': %w", selfexe, err)
	}

	gzname := "bin/" + name + ".gz"
	bin, embedded := BinData[gzname]
	if !embedded {
		logrus.Debug("Skipping not embedded file:", gzname)
		return nil
	}
	logrus.Debugf("%s is at offset %d", gzname, bin.offset)

	if !EmbeddedBinaryNeedsUpdate(exinfo, p, bin.originalSize) {
		logrus.Debug("Re-use existing file:", p)
		return nil
	}

	infile, err := os.Open(selfexe)
	if err != nil {
		return fmt.Errorf("unable to open executable '%s': %w", selfexe, err)
	}
	defer infile.Close()

	// find location at EOF - BinDataSize + offs
	if _, err := infile.Seek(-BinDataSize+bin.offset, 2); err != nil {
		return fmt.Errorf("failed to find embedded file position for '%s': %w", p, err)
	}
	gz, err := gzip.NewReader(io.LimitReader(infile, bin.size))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for '%s': %w", p, err)
	}

	logrus.Debugf("Writing static file: '%s'", p)

	if err := copyTo(p, gz); err != nil {
		return fmt.Errorf("unable to copy to '%s': %w", p, err)
	}
	if err := os.Chmod(p, 0550); err != nil {
		return fmt.Errorf("failed to chmod '%s': %w", p, err)
	}

	// In order to properly determine if an update of an embedded binary file is needed,
	// the staged embedded binary needs to have the same modification time as the `k0s`
	// executable.
	if err := os.Chtimes(p, exinfo.ModTime(), exinfo.ModTime()); err != nil {
		return fmt.Errorf("failed to set file modification times of '%s': %w", p, err)
	}
	return nil
}

func copyTo(p string, gz io.Reader) error {
	_ = os.Remove(p)
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", p, err)
	}
	defer f.Close()
	_, err = io.Copy(f, gz)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %w", p, err)
	}
	return nil
}
