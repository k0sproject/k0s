// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package download

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path/filepath"
	"time"

	internalhttp "github.com/k0sproject/k0s/internal/http"
	"github.com/k0sproject/k0s/internal/pkg/file"
)

type Downloader interface {
	Download(ctx context.Context) error
}

type Config struct {
	URL          string
	ExpectedHash string
	Hasher       hash.Hash
	DownloadDir  string
	Filename     string
}

type downloader struct {
	config Config
}

var _ Downloader = (*downloader)(nil)

func NewDownloader(config Config) Downloader {
	return &downloader{
		config: config,
	}
}

// Performs the download process.
func (d *downloader) Download(ctx context.Context) (err error) {
	var targets []io.Writer

	// If we've been provided a hash and actual value to compare with, use it.
	var expectedHash []byte
	if d.config.Hasher != nil && d.config.ExpectedHash != "" {
		expectedHash, err = hex.DecodeString(d.config.ExpectedHash)
		if err != nil {
			return fmt.Errorf("invalid update hash: %w", err)
		}
		targets = append(targets, d.config.Hasher)
	}

	fileName := "download"
	var downloadOpts []internalhttp.DownloadOption
	if d.config.Filename == "" {
		downloadOpts = append(downloadOpts, internalhttp.StoreSuggestedRemoteFileNameInto(&fileName))
	} else {
		fileName = filepath.Base(d.config.Filename)
		if fileName != d.config.Filename {
			return fmt.Errorf("filename contains path elements: %s", d.config.Filename)
		}
		fileName = d.config.Filename
	}

	// Set up target file for download.
	target, err := file.AtomicWithTarget(filepath.Join(d.config.DownloadDir, fileName)).Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, target.Close()) }()
	targets = append(targets, target)

	// Set a very long overall download timeout. This will ensure that the
	// download will fail at some point, even if the remote server is
	// artificially slow.
	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	// Download from URL into targets.
	if err = internalhttp.Download(ctx, d.config.URL, io.MultiWriter(targets...), downloadOpts...); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Check the hash of the downloaded data and fail if it doesn't match.
	if expectedHash != nil {
		if downloadedHash := d.config.Hasher.Sum(nil); !bytes.Equal(expectedHash, downloadedHash) {
			return fmt.Errorf("hash mismatch: expected %x, got %x", expectedHash, downloadedHash)
		}
	}

	// All is well. Finish the download.
	if err := target.FinishWithBaseName(fileName); err != nil {
		return fmt.Errorf("failed to finish download: %w", err)
	}

	return nil
}
