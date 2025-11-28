//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"net"
	"net/http"
	"os"
)

func newStatusListener(socketPath string) (net.Listener, error) {
	removeLeftovers(socketPath)
	return net.Listen("unix", socketPath)
}

func cleanupStatusListener(socketPath string) {
	_ = os.Remove(socketPath)
}

func removeLeftovers(socket string) {
	_, err := net.Dial("unix", socket)
	if err != nil {
		_ = os.Remove(socket)
	}
}

func newStatusHTTPClient(socketPath string) (*http.Client, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}

	return &http.Client{Transport: transport}, nil
}
