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

package supervisor

import (
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

type pingPong struct {
	shellPath string
	shellArgs []string
	ping      net.Listener
	pong      string
}

func makePingPong(t *testing.T) *pingPong {
	shellPath, err := exec.LookPath("powershell")
	require.NoError(t, err)

	// We need that copy, otherwise tests get cached.
	scriptPath := filepath.Join(t.TempDir(), "pingpong.ps1")
	require.NoError(t, file.Copy("pingpong.ps1", scriptPath))

	guid, err := guid.NewV4()
	require.NoError(t, err)
	namespace := t.Name() + "_" + guid.String()

	// filepath.Clean is broken for Win32 Device Namespaces ...
	// https://github.com/golang/go/issues/23467
	pingPath := `\\.\` + filepath.Join("pipe", namespace, "ping")
	pongPath := `\\.\` + filepath.Join("pipe", namespace, "pong")

	ping, err := winio.ListenPipe(pingPath, nil)
	require.NoError(t, err, "Failed to listen ping pipe")
	t.Cleanup(func() { assert.NoError(t, ping.Close(), "Failed to close ping pipe") })

	return &pingPong{
		shellPath, []string{"-noprofile", "-noninteractive", scriptPath, namespace},
		ping, pongPath,
	}
}

func (pp *pingPong) binPath() string {
	return pp.shellPath
}

func (pp *pingPong) binArgs() []string {
	return pp.shellArgs
}

func (pp *pingPong) awaitPing() (err error) {
	conn, err := pp.ping.Accept()
	if err != nil {
		return err
	}
	defer func() { err = multierr.Append(err, conn.Close()) }()

	_, err = io.ReadAll(conn)
	return err
}

func (pp *pingPong) sendPong() (err error) {
	conn, err := winio.DialPipe(pp.pong, nil)
	if err != nil {
		return err
	}
	defer func() { err = multierr.Append(err, conn.Close()) }()

	_, err = conn.Write([]byte("pong\n"))
	return err
}
