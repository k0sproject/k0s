//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"net"
	"net/http"

	"github.com/Microsoft/go-winio"
)

func newStatusListener(pipePath string) (net.Listener, error) {
	return winio.ListenPipe(pipePath, nil)
}

func cleanupStatusListener(string) {}

func newStatusHTTPClient(pipePath string) (*http.Client, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, pipePath)
		},
	}
	return &http.Client{Transport: transport}, nil
}
