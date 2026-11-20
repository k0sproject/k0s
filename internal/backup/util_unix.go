//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	internalio "github.com/k0sproject/k0s/internal/io"
	"github.com/k0sproject/k0s/internal/pkg/dir"
)

// createArchive compresses and adds files to the backup archive file
func createArchive(ctx context.Context, archive io.Writer, files []string, baseDir string) error {
	gw := gzip.NewWriter(archive)
	tw := tar.NewWriter(internalio.WriterFunc(func(p []byte) (int, error) {
		if err := context.Cause(ctx); err != nil {
			return 0, err
		}
		return gw.Write(p)
	}))

	// Iterate over files and add them to the tar archive
	for _, file := range files {
		err := addToArchive(tw, file, baseDir)
		if err != nil {
			return fmt.Errorf("failed to add path %s: %w", file, err)
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return gw.Close()
}

func addToArchive(tw *tar.Writer, filename string, baseDir string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
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
		if _, err = io.Copy(tw, file); err != nil {
			return err
		}
	}

	return tw.Flush() // detect short reads early
}
