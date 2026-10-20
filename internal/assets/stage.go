// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
)

var errNoPayloadAttached = errors.New("no payload attached")
var errNotEmbedded = errors.New("not an embedded asset")

// StageOpt configures how an executable is staged.
// It receives an AtomicOpener and returns a modified version with additional options.
// This is wrapping the AtomicOpener so we can use variadic options when staging an executable,
// without having to expose the AtomicOpener itself in the assets package.
//
// Example usage with SELinux labels:
//
//	path, err := StageExecutable(dir, "containerd",
//		WithSELinuxLabel("system_u:object_r:container_runtime_exec_t:s0"))
type StageOpt = func(*file.AtomicOpener) *file.AtomicOpener

// WithSELinuxLabel returns a StageOpt that applies the given SELinux label
// to the staged executable. The label should be a complete SELinux context
// in the format user:role:type:level (e.g., "system_u:object_r:container_runtime_exec_t:s0").
// The label will only be applied if SELinux is enabled on the system.
func WithSELinuxLabel(label string) StageOpt {
	return func(o *file.AtomicOpener) *file.AtomicOpener {
		return o.WithSELinuxLabel(label)
	}
}

// Stages the embedded executable with the given name into dir. If the
// executable is not embedded in the k0s executable, this function first checks
// if an executable exists at the desired path. If not, it falls back to a PATH
// lookup. Returns the path to the executable, even if an error occurs.
func StageExecutable(dir, name string, opts ...StageOpt) (string, error) {
	// Always returning the path, even under error conditions, is kind of a hack
	// to work around the "running executable" problem on Windows.

	executableName := name + constant.ExecutableSuffix
	path := filepath.Join(dir, executableName)
	err := stage(executableName, path, 0750, opts...)
	if err == nil {
		return path, nil
	}

	// If the executable is not embedded, try to find an existing one.
	if !errors.Is(err, errNoPayloadAttached) && !errors.Is(err, errNotEmbedded) {
		return path, err
	}

	// First, check if the destination path exists and is not a directory.
	stat, statErr := os.Stat(path)
	if statErr == nil {
		if !stat.IsDir() {
			logrus.WithField("path", path).WithError(err).Debug("Using existing executable")
			return path, nil
		}

		statErr = fmt.Errorf("%w (%s)", syscall.EISDIR, path)
	}

	// If we still haven't found the executable, look for it in the PATH.
	// Don't pass in the executable suffix here, so that the PathExt environment
	// variable works as expected on Windows.
	lookedUpPath, lookErr := exec.LookPath(name)
	if lookErr == nil {
		logrus.WithField("path", lookedUpPath).WithError(err).Debug("Executable found in PATH")
		return lookedUpPath, nil
	}

	return path, fmt.Errorf("%w, %w, %w", err, statErr, lookErr)
}

func stage(name, path string, perm os.FileMode, opts ...StageOpt) error {
	log := logrus.WithField("path", path)
	log.Infof("Staging")

	selfexe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine current executable: %w", err)
	}

	exinfo, err := os.Stat(selfexe)
	if err != nil {
		return fmt.Errorf("unable to stat current executable: %w", err)
	}

	zipFile, err := zip.OpenReader(selfexe)
	if err != nil {
		// If the error indicates an invalid ZIP file, we assume that this is a
		// bare k0s executable, without any ZIP payload appended.
		if errors.Is(err, zip.ErrFormat) {
			return errNoPayloadAttached
		}
		return fmt.Errorf("while staging %q: %w", name, err)
	}
	defer func() { err = errors.Join(err, zipFile.Close()) }()

	var (
		fileToExtract *zip.File
		fileInfo      os.FileInfo
	)
	for _, archivedFile := range zipFile.File {
		if archivedFile.Name == name {
			fileToExtract = archivedFile
			fileInfo = fileToExtract.FileInfo()
			break
		}
	}
	if fileToExtract == nil || fileInfo.IsDir() {
		return errNotEmbedded
	}

	// Skip extraction if the path is up to date, i.e. if its modification time
	// matches the one of the k0s executable and its file size matches the one
	// of the to-be-staged file.
	if info, err := os.Stat(path); err == nil {
		if !exinfo.IsDir() && exinfo.ModTime().Equal(info.ModTime()) && info.Size() == fileInfo.Size() {
			log.Debug("Re-use existing file")
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		// If the error doesn't indicate a non-existing path, then it's likely
		// that the asset can't be staged anyways, so be fail-fast.
		return err
	}

	// Get a reader for the uncompressed file contents
	contents, err := fileToExtract.Open()
	if err != nil {
		if errors.Is(err, zip.ErrAlgorithm) {
			err = fmt.Errorf("%w (compression method %d)", err, fileToExtract.Method)
		}
		return fmt.Errorf("while extracting %q: %w", name, err)
	}
	defer func() { err = errors.Join(err, contents.Close()) }()

	log.Debug("Writing static file")

	opener := file.AtomicWithTarget(path).
		WithPermissions(perm).
		// In order to properly determine if an update of an embedded binary
		// file is needed, the staged embedded binary needs to have the same
		// modification time as the `k0s` executable.
		WithModificationTime(exinfo.ModTime())

	// Apply any additional options
	for _, opt := range opts {
		opener = opt(opener)
	}

	return opener.Do(func(dst file.AtomicWriter) error {
		_, err := io.Copy(dst, contents)
		return err
	})
}
