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

package file

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/users"

	"go.uber.org/multierr"
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
func Chown(file, owner string, permissions os.FileMode) error {
	// Chown the file properly for the owner
	uid, _ := users.GetUID(owner)
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
	defer multierr.AppendInvoke(&err, multierr.Close(in))

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

// WriteAtomically will atomically create or replace a file. The contents of the
// file will be those that the write callback writes to the Writer that gets
// passed in. The Writer will be unbuffered. WriteAtomically will buffer the
// contents in a hidden (i.e. its name will start with a dot), temporary file
// (it will have a .tmp extension). When write returns without an error, the
// temporary file will be renamed to fileName, otherwise it will be deleted
// without touching the target file.
//
// Note that this function is only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func WriteAtomically(fileName string, perm os.FileMode, write func(file io.Writer) error) (err error) {
	var fd *os.File
	fd, err = os.CreateTemp(filepath.Dir(fileName), fmt.Sprintf(".%s.*.tmp", filepath.Base(fileName)))
	if err != nil {
		return err
	}

	tmpFileName := fd.Name()
	close := true
	defer func() {
		remove := err != nil
		if close {
			err = multierr.Append(err, fd.Close())
		}
		if remove {
			removeErr := os.Remove(tmpFileName)
			// Don't propagate any fs.ErrNotExist errors. There is no point in
			// doing this, since the desired state is already reached: The
			// temporary file is no longer present on the file system.
			if removeErr != nil && !errors.Is(err, fs.ErrNotExist) {
				err = multierr.Append(err, removeErr)
			}
		}
	}()

	err = write(fd)
	if err != nil {
		return err
	}

	// https://github.com/google/renameio/blob/v2.0.0/tempfile.go#L150-L157
	err = fd.Sync()
	if err != nil {
		return err
	}

	err = fd.Close()
	close = false
	if err != nil {
		return err
	}

	err = os.Chmod(tmpFileName, perm)
	if err != nil {
		return err
	}

	err = os.Rename(tmpFileName, fileName)
	if err != nil {
		return err
	}

	return nil
}

// WriteContentAtomically will atomically create or replace a file with the
// given content. WriteContentAtomically will create a hidden (i.e. its name
// will start with a dot), temporary file (it will have a .tmp extension) with
// the given content. Afterwards, the temporary file will be renamed to
// fileName, otherwise it will be deleted without touching the target file.
//
// Note that this function is only best-effort on Windows:
// https://github.com/golang/go/issues/22397#issuecomment-498856679
func WriteContentAtomically(fileName string, content []byte, perm os.FileMode) error {
	return WriteAtomically(fileName, perm, func(file io.Writer) error {
		_, err := file.Write(content)
		return err
	})
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
