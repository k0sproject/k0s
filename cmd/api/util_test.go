/*
Copyright 2026 k0s authors

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
