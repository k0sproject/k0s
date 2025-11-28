//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio"
)

func newStatusListener(socketPath string) (net.Listener, error) {
	return winio.ListenPipe(pipePath(socketPath), nil)
}

func cleanupStatusListener(string) {}

func newStatusHTTPClient(socketPath string) (*http.Client, error) {
	path := pipePath(socketPath)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, path)
		},
	}
	return &http.Client{Transport: transport}, nil
}

func pipePath(socketPath string) string {
	if hasPipePrefix(socketPath) {
		return socketPath
	}

	cleaned := filepath.Clean(socketPath)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "-")
	cleaned = strings.ReplaceAll(cleaned, ":", "")

	if cleaned == "" || cleaned == "." {
		cleaned = "k0s-status"
	}

	return `\\.\pipe\` + cleaned
}

func hasPipePrefix(socketPath string) bool {
	if socketPath == "" {
		return false
	}
	lower := strings.ToLower(socketPath)
	return strings.HasPrefix(lower, `\\.\pipe\`) ||
		strings.HasPrefix(lower, `//./pipe/`) ||
		strings.HasPrefix(lower, `\\?\pipe\`)
}
