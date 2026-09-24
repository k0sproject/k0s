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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtcd_FixSocketMode(t *testing.T) {
	// Use a relative socket path to stay below the unix socket path length limit.
	t.Chdir(t.TempDir())
	socketPath := "localhost:2379"

	listen := func() net.Listener {
		l, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		require.NoError(t, os.Chmod(socketPath, 0600))
		return l
	}

	requireMode := func(mode os.FileMode, msgAndArgs ...any) {
		t.Helper()
		info, err := os.Stat(socketPath)
		require.NoError(t, err)
		require.Equal(t, mode, info.Mode().Perm(), msgAndArgs...)
	}

	e := &Etcd{K0sVars: &config.CfgVars{EtcdSocketPath: socketPath}}

	t.Run("adjusts the mode", func(t *testing.T) {
		l := listen()
		t.Cleanup(func() { l.Close() })

		require.NoError(t, e.fixSocketMode(t.Context()))
		requireMode(etcdSocketMode, "socket mode should be adjusted")
	})

	t.Run("waits for the socket to appear", func(t *testing.T) {
		// Simulate etcd being restarted by the supervisor: the socket doesn't
		// exist when the hook runs, and shows up a little later.
		var l net.Listener
		t.Cleanup(func() {
			if l != nil {
				l.Close()
			}
		})
		time.AfterFunc(500*time.Millisecond, func() { l = listen() })

		require.NoError(t, e.fixSocketMode(t.Context()))
		requireMode(etcdSocketMode, "socket mode should be adjusted after the socket appeared")
	})

	t.Run("returns when canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		done := make(chan struct{})
		go func() { defer close(done); assert.NoError(t, e.fixSocketMode(ctx)) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			require.Fail(t, "fixSocketMode didn't return on a canceled context")
		}
	})
}

func TestEtcd_RemoveStaleSocket(t *testing.T) {
	t.Chdir(t.TempDir())
	e := &Etcd{K0sVars: &config.CfgVars{EtcdSocketPath: "localhost:2379"}}

	t.Run("no socket", func(t *testing.T) {
		assert.NoError(t, e.removeStaleSocket())
	})

	t.Run("leftover socket", func(t *testing.T) {
		l, err := net.Listen("unix", e.K0sVars.EtcdSocketPath)
		require.NoError(t, err)
		t.Cleanup(func() { l.Close() })

		require.NoError(t, e.removeStaleSocket())
		_, err = os.Lstat(e.K0sVars.EtcdSocketPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestEnsureUnixSocketMode(t *testing.T) {
	t.Run("refuses symlinks", func(t *testing.T) {
		// Use relative socket paths to stay below the unix socket path length limit.
		t.Chdir(t.TempDir())
		const targetPath, linkPath = "target", "link"
		l, err := net.Listen("unix", targetPath)
		require.NoError(t, err)
		t.Cleanup(func() { l.Close() })
		require.NoError(t, os.Chmod(targetPath, 0600))

		require.NoError(t, os.Symlink(targetPath, linkPath))

		changed, err := ensureUnixSocketMode(linkPath, 0666)
		assert.Error(t, err, "symlinks must not be followed")
		assert.False(t, changed)

		info, err := os.Stat(targetPath)
		if assert.NoError(t, err) {
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "symlink target mode must be unchanged")
		}
	})

	t.Run("refuses non-sockets", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(filePath, nil, 0600))

		changed, err := ensureUnixSocketMode(filePath, 0666)
		assert.Error(t, err, "regular files must not be chmodded")
		assert.False(t, changed)

		info, err := os.Stat(filePath)
		if assert.NoError(t, err) {
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file mode must be unchanged")
		}
	})

	t.Run("missing socket", func(t *testing.T) {
		changed, err := ensureUnixSocketMode(filepath.Join(t.TempDir(), "nonexistent"), 0666)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.False(t, changed)
	})
}

func TestEnsureURLInList(t *testing.T) {
	const socketURL = "unixs:///run/k0s/etcd/localhost:2379"

	for _, test := range []struct {
		name, urls, expected string
	}{
		{"empty list", "", socketURL},
		{"already present", socketURL, socketURL},
		{"present among others", "https://127.0.0.1:2379," + socketURL, "https://127.0.0.1:2379," + socketURL},
		{"appended when missing", "https://0.0.0.0:2379", "https://0.0.0.0:2379," + socketURL},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, ensureURLInList(socketURL, test.urls))
		})
	}
}
