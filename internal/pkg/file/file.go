// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Exists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func Exists(fileName string) bool {
	info, err := os.Stat(fileName)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Chown changes file/dir mode
func Chown(file string, uid int, permissions os.FileMode) error {
	// Chown the file properly for the owner
	err := os.Chown(file, uid, -1)
	if err != nil && os.Geteuid() == 0 {
		return err
	}
	err = os.Chmod(file, permissions)
	if err != nil && os.Geteuid() == 0 {
		return err
	}
	return nil
}

// Copy copies file from src to dst
func Copy(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()

	sourceFileStat, err := in.Stat()
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	return WriteAtomically(dst, sourceFileStat.Mode(), func(out io.Writer) error {
		_, err = io.Copy(out, in)
		return err
	})
}

// WriteNew creates a new file at name with the given permissions and writes
// data to it. It returns an error wrapping [fs.ErrExist] if the file already
// exists, so callers can use errors.Is(err, fs.ErrExist) to detect that case.
func WriteNew(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	return errors.Join(writeErr, f.Close())
}

func WriteTmpFile(data string, prefix string) (path string, err error) {
	tmpFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", fmt.Errorf("cannot create temporary file: %w", err)
	}

	text := []byte(data)
	if _, err = tmpFile.Write(text); err != nil {
		return "", fmt.Errorf("failed to write to temporary file: %w", err)
	}

	return tmpFile.Name(), nil
}
