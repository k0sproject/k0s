// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/internal/pkg/file"
)

func main() {
	log.Println("Extracting.")
	if err := foo(); err != nil {
		log.Fatal(err)
	}
	log.Println("Extraction completed.")
}

func foo() (err error) {
	zipFilePath := "combined.zip" // Path to the zip archive file

	// Open the zip archive for reading

	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer func() { err = errors.Join(err, zipFile.Close()) }()

	zipFile.RegisterDecompressor(12, func(r io.Reader) io.ReadCloser {
		return io.NopCloser(bzip2.NewReader(r))
	})

	// Create a directory to extract the files into
	destDir := "xoxo.xtract"
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("while trying to create destination directory: %w", err)
	}

	// Extract each file from the zip archive
	for _, archivedFile := range zipFile.File {
		destPath := filepath.Join(destDir, filepath.FromSlash(archivedFile.Name))

		info := archivedFile.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("while extracting directory %q: %w", destPath, err)
			}
		} else {
			// Get a reader for the uncompressed file contents
			contents, err := archivedFile.Open()
			if err != nil {
				if errors.Is(err, zip.ErrAlgorithm) {
					err = fmt.Errorf("%w (compression method %d)", err, archivedFile.Method)
				}
				return fmt.Errorf("while extracting file %q: %w", destPath, err)
			}
			defer func() { err = errors.Join(err, contents.Close()) }()

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("while extracting file %q: %w", destPath, err)
			}

			// Write the contents to the destination file.
			err = file.WriteAtomically(destPath, info.Mode(), func(file io.Writer) error {
				// Copy the decompressed data to the destination file while calculating the CRC32 hash
				bytesWritten, err := io.Copy(file, contents)
				if err != nil {
					return fmt.Errorf("while extracting %q: %w", destPath, err)
				}
				if size := info.Size(); bytesWritten != size {
					return fmt.Errorf("file size mismatch for %q: want %d, got %d", destPath, size, bytesWritten)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("while extracting file %q: %w", destPath, err)
			}
		}

		if err := os.Chtimes(destPath, info.ModTime(), info.ModTime()); err != nil {
			log.Printf("Failed to change file times for %q: %v", destPath, err)
		}

		log.Println("Extracted:", destPath)
	}

	return nil
}
