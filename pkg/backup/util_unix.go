//go:build unix

/*
Copyright 2021 k0s authors

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

package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/dir"
)

const timeStampLayout = "2006-01-02T15_04_05_000Z"

// createArchive compresses and adds files to the backup archive file
func createArchive(archive io.Writer, files []string, baseDir string) error {
	gw := gzip.NewWriter(archive)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file, baseDir)
		if err != nil {
			return fmt.Errorf("failed to add file to backup archive: %w", err)
		}
	}
	return nil
}

func addToArchive(tw *tar.Writer, filename string, baseDir string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to fetch file info: %w", err)
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}
	if strings.Contains(filename, baseDir) {
		// calculate relative path of items inside the archive
		rel, err := filepath.Rel(baseDir, filename)
		if err != nil {
			return fmt.Errorf("failed to fetch relative path in tar archive: %w", err)
		}
		header.Name = rel
	}

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("failed to write file header to archive: %w", err)
	}

	if !dir.IsDirectory(filename) {
		// Copy file content to tar archive
		_, err = io.Copy(tw, file)
		if err != nil {
			return fmt.Errorf("failed to copy file contents info archive: %w", err)
		}
	}
	return nil
}

func timeStamp() string {
	return time.Now().Format(timeStampLayout)
}
