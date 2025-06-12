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

package http_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	internalhttp "github.com/k0sproject/k0s/internal/http"
	internalio "github.com/k0sproject/k0s/internal/io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownload_CancelRequest(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.TODO())
	cancel(assert.AnError)

	err := internalhttp.Download(ctx, "http://404.example.com", io.Discard)
	if urlErr := (*url.Error)(nil); assert.ErrorAs(t, err, &urlErr) {
		assert.Equal(t, "Get", urlErr.Op)
		assert.Equal(t, "http://404.example.com", urlErr.URL)
		assert.Equal(t, assert.AnError, urlErr.Err)
	}
}

func TestDownload_NoContent(t *testing.T) {
	baseURL := startFakeDownloadServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	err := internalhttp.Download(context.TODO(), baseURL, io.Discard)
	assert.ErrorContains(t, err, "bad status: 204 No Content")
}

func TestDownload_ShortDownload(t *testing.T) {
	baseURL := startFakeDownloadServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", "10")
		_, err := w.Write([]byte("too short")) // this is only 9 bytes
		assert.NoError(t, err)
	}))

	err := internalhttp.Download(context.TODO(), baseURL, io.Discard)
	assert.ErrorContains(t, err, "while downloading: unexpected EOF")
}

func TestDownload_ExcessContentLength(t *testing.T) {
	baseURL := startFakeDownloadServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", "4")
		_, err := w.Write([]byte("yolo"))
		assert.NoError(t, err)
		// Excess content length
		_, err = w.Write([]byte("<-stripped"))
		assert.ErrorIs(t, err, http.ErrContentLength)
	}))

	var downloaded strings.Builder
	err := internalhttp.Download(context.TODO(), baseURL, &downloaded)

	assert.NoError(t, err)
	assert.Equal(t, "yolo", downloaded.String())
}

func TestDownload_CancelDownload(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.TODO())
	t.Cleanup(func() { cancel(nil) })

	baseURL := startFakeDownloadServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for {
			if _, err := w.Write([]byte(t.Name())); !assert.NoError(t, err) {
				return
			}

			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Microsecond):
			}
		}
	}))

	err := internalhttp.Download(ctx, baseURL, internalio.WriterFunc(func(p []byte) (int, error) {
		cancel(assert.AnError)
		return len(p), nil
	}))

	assert.ErrorContains(t, err, "while downloading: ")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestDownload_RedirectLoop(t *testing.T) {
	// The current implementation doesn't detect loops, but it stops after 10 redirects.

	expectedRequests := uint32(10)
	var requests atomic.Uint32
	var baseURL string
	baseURL = startFakeDownloadServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.LessOrEqual(t, requests.Add(1), expectedRequests, "More requests than anticipated") {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Produce redirect loops: /looper-0, /looper-1, /looper-2, /looper-0, ...
		var seq uint8
		if _, lastSeq, found := strings.Cut(r.URL.Path, "/looper-"); found {
			if lastSeq, err := strconv.ParseUint(lastSeq, 10, 4); err == nil && lastSeq < 3 {
				seq = uint8(lastSeq) + 1
			}
		}

		http.Redirect(w, r, fmt.Sprintf("%s/looper-%d", baseURL, seq), http.StatusSeeOther)
	}))

	var downloaded strings.Builder
	err := internalhttp.Download(context.TODO(), baseURL, &downloaded)

	assert.Equal(t, expectedRequests, requests.Load())
	assert.ErrorContains(t, err, "stopped after 10 redirects")
}

func startFakeDownloadServer(t *testing.T, handler http.Handler) string {
	server := &http.Server{Addr: "localhost:0", Handler: handler}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		require.NoError(t, err)
	}

	serverError := make(chan error)
	go func() {
		defer close(serverError)
		serverError <- server.Serve(listener)
	}()

	t.Cleanup(func() {
		err := server.Shutdown(context.Background())
		if !assert.NoError(t, err, "Couldn't shutdown HTTP server") {
			return
		}

		assert.ErrorIs(t, <-serverError, http.ErrServerClosed, "HTTP server terminated unexpectedly")
	})

	return (&url.URL{Scheme: "http", Host: listener.Addr().String()}).String()
}
