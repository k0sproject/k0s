//go:build unix

// SPDX-FileCopyrightText: 2025 k0s authors
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

func TestEtcd_MaintainSocketMode(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "etcd.sock")

	listen := func() net.Listener {
		l, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		require.NoError(t, os.Chmod(socketPath, 0600))
		return l
	}

	hasMode := func(mode os.FileMode) func() bool {
		return func() bool {
			info, err := os.Stat(socketPath)
			return err == nil && info.Mode().Perm() == mode
		}
	}

	l := listen()
	t.Cleanup(func() { l.Close() })

	e := &Etcd{K0sVars: &config.CfgVars{EtcdSocketPath: socketPath}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go e.maintainSocketMode(ctx)

	require.Eventually(t, hasMode(etcdSocketMode), 10*time.Second, 10*time.Millisecond,
		"socket mode should be adjusted to %o", etcdSocketMode)

	// Simulate etcd being restarted by the supervisor: the socket is
	// re-created with restrictive permissions and should be adjusted again.
	require.NoError(t, l.Close())
	l = listen()
	require.Eventually(t, hasMode(etcdSocketMode), 10*time.Second, 10*time.Millisecond,
		"socket mode should be adjusted to %o after the socket is re-created", etcdSocketMode)
}
