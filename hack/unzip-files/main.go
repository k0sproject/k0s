//go:build hack

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz/lzma"
)

func main() {
	if err := unzip(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func unzip() error {
	zipFile, err := zip.OpenReader("archive.zip")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, zipFile.Close()) }()

	zipFile.RegisterDecompressor(14, func(r io.Reader) io.ReadCloser {
		r, err := lzma.ReaderConfig{DictCap: min(math.MaxInt, lzma.MaxDictCap)}.NewReader(r)
		if err != nil {
			r = &errReader{err}
		}
		return io.NopCloser(r)
	})
	zipFile.RegisterDecompressor(zstd.ZipMethodWinZip, zstd.ZipDecompressor())

	for _, archivedFile := range zipFile.File {
		fmt.Fprintln(os.Stderr, "Extracting", archivedFile.Name)

		err := func() error {
			contents, err := archivedFile.Open()
			if err != nil {
				if errors.Is(err, zip.ErrAlgorithm) {
					err = fmt.Errorf("%w (compression method %d)", err, archivedFile.Method)
				}
				return fmt.Errorf("while extracting %q: %w", archivedFile.Name, err)
			}
			defer func() { err = errors.Join(err, contents.Close()) }()

			bytesWritten, err := io.Copy(io.Discard, contents)
			if err != nil {
				return fmt.Errorf("while extracting %q: %w", archivedFile.Name, err)
			}
			if size := archivedFile.FileInfo().Size(); bytesWritten != size {
				return fmt.Errorf("file size mismatch for %q: want %d, got %d", archivedFile.Name, size, bytesWritten)
			}
			return nil
		}()

		if err != nil {
			o, e := archivedFile.DataOffset()
			if e != nil {
				o = -1
				err = errors.Join(err, e)
			}
			return fmt.Errorf("%w (data offset: %d, length: %d)", err, o, archivedFile.CompressedSize64)
		}
	}

	return nil
}

type errReader struct{ error }

func (r *errReader) Read(p []byte) (n int, err error) {
	err = r.error
	return
}
