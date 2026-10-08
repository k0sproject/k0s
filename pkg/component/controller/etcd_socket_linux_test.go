//go:build linux

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestEtcdSocketGroupAccess(t *testing.T) {
	if socketPath := os.Getenv("K0S_TEST_ETCD_SOCKET"); socketPath != "" {
		conn, err := net.DialTimeout("unix", socketPath, time.Second)
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root to check access from separate uids and gids")
	}

	runDir, err := os.MkdirTemp("", "etcd-permissions-") //nolint:usetesting // child uids must traverse the parent directory.
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(runDir)) })
	// The directory stays traversable. The socket mode is what allows group 0
	// and refuses every other group.
	require.NoError(t, os.Chmod(runDir, 0755))

	socketPath := filepath.Join(runDir, "etcd.sock")
	e := &Etcd{K0sVars: &config.CfgVars{EtcdSocketPath: socketPath}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go e.maintainSocketMode(ctx)

	// go test leaves its binary in a directory the other uid cannot traverse.
	exe, err := os.Executable()
	require.NoError(t, err)
	bin := filepath.Join(runDir, "socket.test")
	require.NoError(t, copyExecutable(exe, bin))

	oldUmask := syscall.Umask(0077)
	t.Cleanup(func() { syscall.Umask(oldUmask) })
	for range 2 {
		listener, err := net.Listen("unix", socketPath)
		require.NoError(t, err)
		require.NoError(t, os.Chown(socketPath, 0, 0))
		require.Eventually(t, func() bool {
			info, err := os.Stat(socketPath)
			return err == nil && info.Mode().Perm() == etcdSocketMode
		}, 5*time.Second, 10*time.Millisecond)
		for _, gid := range []uint32{0, 65534} {
			cmd := exec.Command(bin, "-test.run=^TestEtcdSocketGroupAccess$")
			cmd.Env = append(os.Environ(), "K0S_TEST_ETCD_SOCKET="+socketPath)
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{
				Uid: 65534, Gid: gid,
			}}
			output, err := cmd.CombinedOutput()
			if gid == 0 {
				require.NoError(t, err, "%s", output)
			} else {
				require.Error(t, err, "unrelated users must not connect")
			}
		}
		require.NoError(t, listener.Close())
	}
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
