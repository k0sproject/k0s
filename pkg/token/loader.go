// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	// httpClientTimeout is the maximum time allowed for HTTP requests when fetching tokens
	httpClientTimeout = 90 * time.Second
)

// Load loads token content from a file path or URL.
// Supports plain file paths, file://, http://, and https:// protocols.
// The file:// prefix is automatically stripped and treated as a plain file path.
func Load(ctx context.Context, pathOrURL string, insecure bool) ([]byte, error) {
	// Check if it's an HTTP/HTTPS URL
	if isHTTPURL(pathOrURL) {
		return fetchFromURL(ctx, pathOrURL, insecure)
	}

	// Handle file:// protocol by stripping prefix and treating as file path
	pathOrURL = strings.TrimPrefix(pathOrURL, "file://")

	// Treat as file path
	return os.ReadFile(pathOrURL)
}

// isHTTPURL checks if the given string is an HTTP or HTTPS URL
func isHTTPURL(pathOrURL string) bool {
	return strings.HasPrefix(pathOrURL, "http://") ||
		strings.HasPrefix(pathOrURL, "https://")
}

// fetchFromURL fetches token content from HTTP or HTTPS URL
func fetchFromURL(ctx context.Context, urlStr string, insecure bool) ([]byte, error) {
	// Parse the URL to add hostname query parameter
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Get the hostname of the current node
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Add hostname as query parameter
	query := parsedURL.Query()
	query.Set("hostname", hostname)

	// Add architecture
	query.Set("arch", runtime.GOARCH)

	// Add machine-id if available
	if machineID, err := os.ReadFile("/etc/machine-id"); err == nil {
		// Trim whitespace (machine-id files typically have a trailing newline)
		trimmedID := strings.TrimSpace(string(machineID))
		if trimmedID != "" {
			query.Set("machine-id", trimmedID)
		}
	}

	parsedURL.RawQuery = query.Encode()
	urlStr = parsedURL.String()

	client := &http.Client{
		Timeout: httpClientTimeout,
	}
	if insecure && strings.HasPrefix(urlStr, "https://") {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: HTTP 404 Not Found", os.ErrNotExist)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	tokenBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return tokenBytes, nil
}
