//go:build hack

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/dsnet/compress/bzip2"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := compress(ctx, os.Stdout, os.Args[1:])
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

type compressedFile struct {
	header zip.FileHeader
	buf    bytes.Buffer
}

func compress(ctx context.Context, w io.Writer, filePaths []string) (err error) {
	slices.SortFunc(filePaths, func(l, r string) int {
		return strings.Compare(filepath.Base(l), filepath.Base(r))
	})
	for i, filePath := range filePaths[1:] {
		l, r := filePaths[i], filePath
		if filepath.Base(l) == filepath.Base(r) {
			return fmt.Errorf("duplicate file name for paths %q and %q", l, r)
		}
	}

	compressedFiles := make([]*compressedFile, len(filePaths))
	g, ctx := errgroup.WithContext(ctx)
	for i := range filePaths {
		g.Go(func() error {
			f, err := compressFile(ctx, filePaths[i])
			if err != nil {
				return err
			}

			h := &f.header
			fmt.Fprintf(os.Stderr, "Compressed %s: %.1f MiB -> %.1f MiB (%.0f%%)\n",
				h.Name,
				float64(h.UncompressedSize64)/1024/1024,
				float64(h.CompressedSize64)/1024/1024,
				100*(1-(float64(h.CompressedSize64)/float64(h.UncompressedSize64))),
			)
			compressedFiles[i] = f
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	archive := zip.NewWriter(w)
	defer func() { err = errors.Join(err, archive.Close()) }()

	for _, f := range compressedFiles {
		archived, err := archive.CreateRaw(&f.header)
		if err != nil {
			return err
		}
		bytesWritten, err := f.buf.WriteTo(archived)
		if err != nil {
			return err
		}
		if uint64(bytesWritten) != f.header.CompressedSize64 {
			return fmt.Errorf("file size mismatch: want %d, got %d", f.header.CompressedSize64, bytesWritten)
		}
	}

	return nil
}

func compressFile(ctx context.Context, filePath string) (*compressedFile, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()

	var f compressedFile
	compressed, err := bzip2.NewWriter(&f.buf, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if err != nil {
		return nil, err
	}

	crc32 := crc32.NewIEEE()
	bytesWritten, err := io.Copy(io.MultiWriter(compressed, crc32), &guardedReader{file, ctx.Err})
	if err != nil {
		return nil, err
	}
	if err := compressed.Close(); err != nil {
		return nil, err
	}

	f.header.Name = filepath.Base(filePath)
	f.header.UncompressedSize64 = uint64(bytesWritten)
	f.header.CompressedSize64 = uint64(f.buf.Len())
	f.header.Method = 12 // this is bzip2
	f.header.CRC32 = crc32.Sum32()

	return &f, nil
}

type guardedReader struct {
	delegate io.Reader
	guard    func() error
}

func (r *guardedReader) Read(p []byte) (n int, err error) {
	if err := r.guard(); err != nil {
		return 0, err
	}
	return r.delegate.Read(p)
}
