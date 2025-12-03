//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func newStatusListener(pipePath string) (net.Listener, error) {
	return winio.ListenPipe(pipePath, nil)
}

func cleanupStatusListener(string) {}

func dialSocket(ctx context.Context, socketPath string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, socketPath)
}
