//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestEtcdMaintainSocketMode(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "etcd.sock")
	listen := func() net.Listener {
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		require.NoError(t, os.Chmod(socketPath, 0600))
		return listener
	}
	mode := func() os.FileMode {
		info, err := os.Stat(socketPath)
		require.NoError(t, err)
		return info.Mode().Perm()
	}

	listener := listen()
	t.Cleanup(func() { _ = listener.Close() })

	e := &Etcd{K0sVars: &config.CfgVars{EtcdSocketPath: socketPath}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go e.maintainSocketMode(ctx)

	require.Eventually(t, func() bool { return mode() == etcdSocketMode }, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, listener.Close())
	listener = listen()
	require.Eventually(t, func() bool { return mode() == etcdSocketMode }, 5*time.Second, 10*time.Millisecond)
}
