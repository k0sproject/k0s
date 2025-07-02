// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package pingpong

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

type StartOptions struct {
	Env                           []string
	IgnoreGracefulShutdownRequest bool
}

func Start(t *testing.T, opts ...StartOptions) (*exec.Cmd, *PingPong) {
	pingPong := New(t)
	for _, opt := range opts {
		pingPong.IgnoreGracefulShutdownRequest = opt.IgnoreGracefulShutdownRequest
	}
	cmd := exec.Command(pingPong.BinPath(), pingPong.BinArgs()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	for _, opt := range opts {
		cmd.Env = opt.Env
	}
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _, _ = cmd.Process.Kill(), cmd.Wait() })
	return cmd, pingPong
}
