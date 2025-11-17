// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/token"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_PlainFilePath(t *testing.T) {
	t.Parallel()

	// Create a temporary file with token content
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	tokenContent := []byte("test-token-content")
	require.NoError(t, os.WriteFile(tokenFile, tokenContent, 0o600))

	// Test loading from plain file path
	result, err := token.Load(context.Background(), tokenFile, false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_FileProtocol(t *testing.T) {
	t.Parallel()

	// Create a temporary file with token content
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	tokenContent := []byte("test-token-content")
	require.NoError(t, os.WriteFile(tokenFile, tokenContent, 0o600))

	// Test loading with file:// protocol
	result, err := token.Load(context.Background(), "file://"+tokenFile, false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_FileNotFound(t *testing.T) {
	t.Parallel()

	// Test loading from non-existent file
	_, err := token.Load(context.Background(), "/nonexistent/path/to/token", false)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoad_HTTPSuccess(t *testing.T) {
	t.Parallel()

	tokenContent := []byte("http-token-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	defer server.Close()

	result, err := token.Load(context.Background(), server.URL, false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_HTTPQueryParameters_AutomaticParams(t *testing.T) {
	t.Parallel()

	// Get expected values
	expectedHostname, err := os.Hostname()
	require.NoError(t, err)
	expectedArch := runtime.GOARCH

	// Check if machine-id is available
	machineIDBytes, err := os.ReadFile("/etc/machine-id")
	hasMachineID := err == nil && len(strings.TrimSpace(string(machineIDBytes))) > 0
	var expectedMachineID string
	if hasMachineID {
		expectedMachineID = strings.TrimSpace(string(machineIDBytes))
	}

	tokenContent := []byte("http-token-content")
	receivedParams := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedParams["hostname"] = r.URL.Query().Get("hostname")
		receivedParams["arch"] = r.URL.Query().Get("arch")
		receivedParams["machine-id"] = r.URL.Query().Get("machine-id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	defer server.Close()

	result, err := token.Load(context.Background(), server.URL, false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)

	// Verify hostname and arch parameters are automatically added
	assert.Equal(t, expectedHostname, receivedParams["hostname"])
	assert.Equal(t, expectedArch, receivedParams["arch"])

	// Verify machine-id is added when available
	if hasMachineID {
		assert.Equal(t, expectedMachineID, receivedParams["machine-id"])
	} else {
		assert.Empty(t, receivedParams["machine-id"])
	}
}

func TestLoad_HTTPS_Success(t *testing.T) {
	t.Parallel()

	tokenContent := []byte("https-token-content")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	defer server.Close()

	// Test with insecure=true (skips cert verification)
	result, err := token.Load(context.Background(), server.URL, true)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_HTTPS_CertVerificationFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	// Test with insecure=false (enforces cert verification)
	// This should fail because httptest uses self-signed certs
	_, err := token.Load(context.Background(), server.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch from URL")
}

func TestLoad_HTTP404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := token.Load(context.Background(), server.URL, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist, "HTTP 404 should be mapped to os.ErrNotExist")
	assert.Contains(t, err.Error(), "HTTP 404 Not Found")
}

func TestLoad_HTTPOtherError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := token.Load(context.Background(), server.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status: 500")
}

func TestLoad_HTTPConnectionRefused(t *testing.T) {
	t.Parallel()

	// Use a URL that will definitely fail to connect
	_, err := token.Load(context.Background(), "http://localhost:1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch from URL")
}

func TestLoad_EmptyFile(t *testing.T) {
	t.Parallel()

	// Create an empty file
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "empty-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte{}, 0o600))

	// Loading empty file should succeed but return empty content
	result, err := token.Load(context.Background(), tokenFile, false)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoad_HTTPEmptyResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Don't write any content
	}))
	defer server.Close()

	result, err := token.Load(context.Background(), server.URL, false)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoad_InsecureFlagOnlyAppliesToHTTPS(t *testing.T) {
	t.Parallel()

	tokenContent := []byte("http-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	defer server.Close()

	// insecure flag should not affect HTTP (only HTTPS)
	result, err := token.Load(context.Background(), server.URL, true)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_URLValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		isHTTP       bool
		errorMessage string
	}{
		// Valid HTTP/HTTPS URLs
		{"http URL", "http://localhost:1", true, "failed to fetch from URL"},
		{"https URL", "https://localhost:1", true, "failed to fetch from URL"},

		// Invalid/malformed HTTP URLs
		{"http no host", "http://", true, "failed to fetch from URL"},
		{"https no host", "https://", true, "failed to fetch from URL"},
		{"http invalid bracket", "http://[invalid", true, "failed to parse URL"},
		{"https invalid port", "https://example.com:invalid-port", true, "failed to parse URL"},

		// Non-HTTP URLs (case sensitive, treated as file paths)
		{"HTTP uppercase", "HTTP://localhost:1", false, ""},
		{"HTTPS uppercase", "HTTPS://localhost:1", false, ""},
		{"file path", "/path/to/file", false, ""},
		{"file protocol", "file:///path/to/file", false, ""},
		{"ftp protocol", "ftp://localhost:1", false, ""},
		{"empty string", "", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := token.Load(context.Background(), tt.input, false)

			if tt.isHTTP {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else if tt.input != "" && tt.input != "/path/to/file" {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "failed to fetch from URL")
			}
		})
	}
}

func TestLoad_HTTPSWithCustomTransport(t *testing.T) {
	t.Parallel()

	tokenContent := []byte("secure-token")

	// Create a server with a custom TLS config
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the client is using TLS
		assert.NotNil(t, r.TLS, "Request should use TLS")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	server.StartTLS()
	defer server.Close()

	// Test with insecure=true (should skip cert verification)
	result, err := token.Load(context.Background(), server.URL, true)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_FileProtocolWithRelativePath(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token")
	tokenContent := []byte("relative-path-token")
	require.NoError(t, os.WriteFile(tokenFile, tokenContent, 0o600))

	// Change to temp directory
	t.Chdir(tmpDir)

	// Test with file:// and relative path
	result, err := token.Load(context.Background(), "file://token", false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_HTTPRedirect(t *testing.T) {
	t.Parallel()

	tokenContent := []byte("redirected-token")

	// Create the target server
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenContent)
	}))
	defer targetServer.Close()

	// Create a redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	// HTTP client should follow redirects by default
	result, err := token.Load(context.Background(), redirectServer.URL, false)
	require.NoError(t, err)
	assert.Equal(t, tokenContent, result)
}

func TestLoad_HTTPStatusCodes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		statusCode   int
		expectError  bool
		errorMessage string
		isNotExist   bool
	}{
		{"200 OK", http.StatusOK, false, "", false},
		{"201 Created", http.StatusCreated, true, "unexpected HTTP status: 201", false},
		{"204 No Content", http.StatusNoContent, true, "unexpected HTTP status: 204", false},
		{"301 Moved Permanently", http.StatusMovedPermanently, true, "unexpected HTTP status: 301", false},
		{"400 Bad Request", http.StatusBadRequest, true, "unexpected HTTP status: 400", false},
		{"401 Unauthorized", http.StatusUnauthorized, true, "unexpected HTTP status: 401", false},
		{"403 Forbidden", http.StatusForbidden, true, "unexpected HTTP status: 403", false},
		{"404 Not Found", http.StatusNotFound, true, "HTTP 404 Not Found", true},
		{"500 Internal Server Error", http.StatusInternalServerError, true, "unexpected HTTP status: 500", false},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true, "unexpected HTTP status: 503", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				if tc.statusCode == http.StatusOK {
					_, _ = w.Write([]byte("token-content"))
				}
			}))
			defer server.Close()

			result, err := token.Load(context.Background(), server.URL, false)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMessage)
				if tc.isNotExist {
					assert.ErrorIs(t, err, os.ErrNotExist)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, []byte("token-content"), result)
			}
		})
	}
}

func TestLoad_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a server that blocks until client cancels
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait for the request context to be canceled
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	// This should fail with context cancellation error
	_, err := token.Load(ctx, server.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestLoad_ContextTimeout(t *testing.T) {
	t.Parallel()

	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("too-late"))
	}))
	defer server.Close()

	// Create a context with a short timeout (less than server delay)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should fail with context deadline exceeded
	_, err := token.Load(ctx, server.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
