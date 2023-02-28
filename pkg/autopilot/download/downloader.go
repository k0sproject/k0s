// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package download

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash"

	"github.com/cavaliergopher/grab/v3"
	"github.com/sirupsen/logrus"
)

type Downloader interface {
	Download(ctx context.Context) error
}

type Config struct {
	URL          string
	ExpectedHash string
	Hasher       hash.Hash
	DownloadDir  string
}

type downloader struct {
	config       Config
	logger       *logrus.Entry
	httpResponse *grab.Response
}

var _ Downloader = (*downloader)(nil)

func NewDownloader(config Config, logger *logrus.Entry) Downloader {
	return &downloader{
		config: config,
		logger: logger.WithField("component", "downloader"),
	}
}

// Start begins the download process, starting the downloading functionality
// on a separate goroutine. Cancelling the context will abort this operation
// once started.
func (d *downloader) Download(ctx context.Context) error {
	// Setup the library for downloading HTTP content ..
	dlreq, err := grab.NewRequest(d.config.DownloadDir, d.config.URL)
	if err != nil {
		return fmt.Errorf("invalid download request: %w", err)
	}

	// If we've been provided a hash and actual value to compare with, use it.
	if d.config.Hasher != nil && d.config.ExpectedHash != "" {
		expectedHash, err := hex.DecodeString(d.config.ExpectedHash)
		if err != nil {
			return fmt.Errorf("invalid update hash: %w", err)
		}

		dlreq.SetChecksum(d.config.Hasher, expectedHash, true)
	}

	client := grab.NewClient()
	d.httpResponse = client.Do(dlreq)

	select {
	case <-d.httpResponse.Done:
		return d.httpResponse.Err()

	case <-ctx.Done():
		return fmt.Errorf("download cancelled")
	}
}
