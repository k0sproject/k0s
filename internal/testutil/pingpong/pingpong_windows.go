/*
Copyright 2023 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pingpong

import (
	_ "embed"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed pingpong.ps1
var script []byte

type PingPong struct {
	IgnoreGracefulShutdownRequest bool // Has no effect on Windows.

	shellPath string
	shellArgs []string
	ping      net.Listener
	pong      string
}

func New(t *testing.T) *PingPong {
	shellPath, err := exec.LookPath("powershell")
	require.NoError(t, err)

	scriptPath := filepath.Join(t.TempDir(), "pingpong.ps1")
	require.NoError(t, os.WriteFile(scriptPath, script, 0700))

	guid, err := guid.NewV4()
	require.NoError(t, err)
	namespace := t.Name() + "_" + guid.String()

	pingPath := filepath.Join(`\\.\pipe`, namespace, "ping")
	pongPath := filepath.Join(`\\.\pipe`, namespace, "pong")

	ping, err := winio.ListenPipe(pingPath, nil)
	require.NoError(t, err, "Failed to listen ping pipe")
	t.Cleanup(func() { assert.NoError(t, ping.Close(), "Failed to close ping pipe") })

	return &PingPong{
		shellPath: shellPath,
		shellArgs: []string{"-noprofile", "-noninteractive", scriptPath, namespace},
		ping:      ping,
		pong:      pongPath,
	}
}

func (pp *PingPong) BinPath() string {
	return pp.shellPath
}

func (pp *PingPong) BinArgs() []string {
	return pp.shellArgs
}

func (pp *PingPong) AwaitPing() (err error) {
	conn, err := pp.ping.Accept()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close()) }()

	_, err = io.ReadAll(conn)
	return err
}

func (pp *PingPong) SendPong() (err error) {
	conn, err := winio.DialPipe(pp.pong, nil)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close()) }()

	_, err = conn.Write([]byte("pong\n"))
	return err
}
