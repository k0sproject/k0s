// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"

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
	log := logrus.WithField("path", p)
	log.Infof("Staging")

	selfexe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine current executable: %w", err)
	}

	exinfo, err := os.Stat(selfexe)
	if err != nil {
		return fmt.Errorf("unable to stat current executable: %w", err)
	}

	gzname := "bin/" + name + ".gz"
	bin, embedded := BinData[gzname]
	if !embedded {
		log.Debug("Skipping not embedded file:", gzname)
		return nil
	}
	log.Debugf("%s is at offset %d", gzname, bin.offset)

	if !EmbeddedBinaryNeedsUpdate(exinfo, p, bin.originalSize) {
		log.Debug("Re-use existing file")
		return nil
	}

	infile, err := os.Open(selfexe)
	if err != nil {
		return fmt.Errorf("unable to open current executable: %w", err)
	}
	defer infile.Close()

	// find location at EOF - BinDataSize + offs
	if _, err := infile.Seek(-BinDataSize+bin.offset, 2); err != nil {
		return fmt.Errorf("failed to find embedded file position for %q: %w", p, err)
	}
	gz, err := gzip.NewReader(io.LimitReader(infile, bin.size))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader for %q: %w", p, err)
	}

	log.Debug("Writing static file")

	return file.AtomicWithTarget(p).
		WithPermissions(0750).
		// In order to properly determine if an update of an embedded binary
		// file is needed, the staged embedded binary needs to have the same
		// modification time as the `k0s` executable.
		WithModificationTime(exinfo.ModTime()).
		Do(func(dst file.AtomicWriter) error {
			_, err := io.Copy(dst, gz)
			return err
		})
}
