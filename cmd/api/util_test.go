// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type failingResponseWriter struct {
	header     http.Header
	statusCode int
	writeCalls int
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	w.writeCalls++
	return 0, errors.New("write failed")
}

func TestSendErrorDoesNotRetryWriteFailure(t *testing.T) {
	resp := &failingResponseWriter{}

	sendError(errors.New("boom"), resp, http.StatusBadRequest)

	assert.Equal(t, "text/plain", resp.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusBadRequest, resp.statusCode)
	assert.Equal(t, 1, resp.writeCalls, "sendError should not recurse on write failure")
}

func TestSendErrorWritesStatusAndBody(t *testing.T) {
	resp := httptest.NewRecorder()

	sendError(errors.New("boom"), resp, http.StatusBadRequest)

	assert.Equal(t, "text/plain", resp.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "boom", resp.Body.String())
}
