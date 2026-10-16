//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchSocketMode(t *testing.T) {
	// Use a relative socket path to stay below the unix socket path length limit.
	t.Chdir(t.TempDir())
	const socketPath = "localhost:2379"
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	listen := func(t *testing.T) {
		l, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		t.Cleanup(func() { l.Close() })
	}

	awaitMode := func(t *testing.T, mode os.FileMode, msg string) {
		t.Helper()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			info, err := os.Stat(socketPath)
			if assert.NoError(t, err) {
				assert.Equal(t, mode, info.Mode().Perm())
			}
		}, 10*time.Second, 10*time.Millisecond, msg)
	}

	t.Run("adjusts the mode of an existing socket", func(t *testing.T) {
		listen(t)
		require.NoError(t, os.Chmod(socketPath, 0600))

		stop, err := watchSocketMode(log, socketPath)
		require.NoError(t, err)
		t.Cleanup(stop)

		awaitMode(t, etcdSocketMode, "socket mode should be adjusted")
	})

	t.Run("adjusts the mode of (re-)created sockets", func(t *testing.T) {
		stop, err := watchSocketMode(log, socketPath)
		require.NoError(t, err)
		t.Cleanup(stop)

		for i := range 2 {
			l, err := net.Listen("unix", socketPath)
			require.NoError(t, err)
			awaitMode(t, etcdSocketMode, fmt.Sprintf("socket mode should be adjusted after socket creation #%d", i+1))
			require.NoError(t, l.Close())
		}
	})

	t.Run("adjusts the mode after chmod", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Only inotify reports mode changes of files in watched directories")
		}
		listen(t)

		stop, err := watchSocketMode(log, socketPath)
		require.NoError(t, err)
		t.Cleanup(stop)
		awaitMode(t, etcdSocketMode, "socket mode should be adjusted")

		require.NoError(t, os.Chmod(socketPath, 0600))
		awaitMode(t, etcdSocketMode, "socket mode should be adjusted after chmod")
	})

	t.Run("stop terminates the watch", func(t *testing.T) {
		stop, err := watchSocketMode(log, socketPath)
		require.NoError(t, err)

		done := make(chan struct{})
		go func() { defer close(done); stop() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			require.Fail(t, "stop didn't return")
		}
	})

	t.Run("fails if the directory doesn't exist", func(t *testing.T) {
		_, err := watchSocketMode(log, filepath.Join("nonexistent", socketPath))
		assert.ErrorIs(t, err, os.ErrNotExist, "watching a nonexistent directory should fail")
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
		assert.False(t, changed, "nothing should be reported as changed")

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
		assert.False(t, changed, "nothing should be reported as changed")

		info, err := os.Stat(filePath)
		if assert.NoError(t, err) {
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file mode must be unchanged")
		}
	})

	t.Run("fails for a missing socket", func(t *testing.T) {
		changed, err := ensureUnixSocketMode(filepath.Join(t.TempDir(), "nonexistent"), 0666)
		assert.ErrorIs(t, err, os.ErrNotExist, "a missing socket should be reported as nonexistent")
		assert.False(t, changed, "nothing should be reported as changed")
	})
}

func TestEnsureURLInList(t *testing.T) {
	const socketURL = "unixs:///run/k0s/etcd/localhost:2379"

	for _, test := range []struct {
		name, urls, expected string
	}{
		{"returns the URL for an empty list", "", socketURL},
		{"keeps a list that only contains the URL", socketURL, socketURL},
		{"keeps a list that contains the URL among others", "https://127.0.0.1:2379," + socketURL, "https://127.0.0.1:2379," + socketURL},
		{"appends the URL if missing", "https://0.0.0.0:2379", "https://0.0.0.0:2379," + socketURL},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, ensureURLInList(socketURL, test.urls), "the URL should be in the list exactly once")
		})
	}
}
