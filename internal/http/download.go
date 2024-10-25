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

package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	internalio "github.com/k0sproject/k0s/internal/io"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/k0scontext"
)

type DownloadOption func(*downloadOptions)

// Downloads the contents of the given URL. Writes the HTTP response body to writer.
// Stalled downloads will be aborted if there's no data transfer for some time.
func Download(ctx context.Context, url string, target io.Writer, options ...DownloadOption) (err error) {
	opts := downloadOptions{
		stalenessTimeout: time.Minute,
		header:           http.Header{},
	}
	for _, opt := range options {
		opt(&opts)
	}

	// this transports is, by design, a trimmed down version of http's
	// DefaultTransport. we do not want to have all those timeouts the
	// default client brings in.
	transport := &http.Transport{
		// This is a one-shot HTTP client which should release resources immediately.
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment,
	}

	// if tls check is disabled, we need to disable the transport's TLS checks as well.
	if opts.insecureSkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// Prepare the client and the request.
	client := http.Client{Transport: transport}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("invalid download request: %w", err)
	}
	req.Header.Set("User-Agent", "k0s/"+build.Version)
	for header, values := range opts.header {
		for _, value := range values {
			req.Header.Add(header, value)
		}
	}

	if opts.username != "" && opts.password != "" {
		req.SetBasicAuth(opts.username, opts.password)
	}

	// Create a context with an inactivity timeout to cancel the download if it stalls.
	ctx, cancel, keepAlive := k0scontext.WithInactivityTimeout(ctx, opts.stalenessTimeout)
	defer cancel(nil)

	// Execute the request.
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	if err := opts.detectRemoteFileName(resp); err != nil {
		return err
	}

	// Monitor writes. Keep the download context alive as long as data is flowing.
	writeMonitor := internalio.WriterFunc(func(p []byte) (int, error) {
		len := len(p)
		if len > 0 {
			keepAlive()
		}
		return len, nil
	})

	// Run the actual data transfer.
	if _, err := io.Copy(io.MultiWriter(writeMonitor, target), resp.Body); err != nil {
		return fmt.Errorf("while downloading: %w", err)
	}

	return nil
}

// Sets the staleness timeout for a download.
// Defaults to one minute if omitted.
func WithStalenessTimeout(stalenessTimeout time.Duration) DownloadOption {
	return func(opts *downloadOptions) {
		opts.stalenessTimeout = stalenessTimeout
	}
}

// WithInsecureSkipTLSVerify sets the insecureSkipTLSVerify option to true.
func WithInsecureSkipTLSVerify() DownloadOption {
	return func(opts *downloadOptions) {
		opts.insecureSkipTLSVerify = true
	}
}

// WithBasicAuth sets the username and password for basic auth.
func WithBasicAuth(username, password string) DownloadOption {
	return func(opts *downloadOptions) {
		opts.username = username
		opts.password = password
	}
}

// WithHeader adds a header to the request.
func WithHeader(header, value string) DownloadOption {
	return func(opts *downloadOptions) {
		opts.header.Add(header, value)
	}
}

type downloadOptions struct {
	stalenessTimeout      time.Duration
	insecureSkipTLSVerify bool
	username              string
	password              string
	header                http.Header
	downloadFileNameOptions
}
