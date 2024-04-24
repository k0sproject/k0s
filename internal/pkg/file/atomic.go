/*
Copyright 2024 k0s authors

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
)

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
	// Determine the absolute path of the file. This is a safeguard to make this
	// function robust against intermediary working directory changes.
	if fileName, err = filepath.Abs(fileName); err != nil {
		return err
	}

	var fd *os.File
	fd, err = os.CreateTemp(filepath.Dir(fileName), fmt.Sprintf(".%s.*.tmp", filepath.Base(fileName)))
	if err != nil {
		return err
	}

	tmpFileName := fd.Name()
	written, close := false, true
	defer func() {
		var errs []error
		if err != nil {
			errs = append(errs, err)
		}

		if close {
			if err := fd.Close(); err != nil {
				errs = append(errs, err)
			}
		}

		if !written || err != nil {
			err := os.Remove(tmpFileName)
			// Don't propagate any fs.ErrNotExist errors. There is no point in
			// doing this, since the desired state is already reached: The
			// temporary file is no longer present on the file system.
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}

		if len(errs) == 1 {
			err = errs[0]
		} else {
			err = errors.Join(errs...)
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

	written = true
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
