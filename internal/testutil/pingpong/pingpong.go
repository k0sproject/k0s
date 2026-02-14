// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package pingpong

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pingPongMarker = "__TESTUTIL_PINGPONG"

type Options struct {
	// Won't respond to graceful termination requests if true.
	IgnoreGracefulTerminationRequests bool
}

type StartOptions struct {
	Options
	Env []string
}

func Start(t *testing.T, opts ...StartOptions) (*exec.Cmd, *PingPong) {
	pingPong := New(t)
	for _, opt := range opts {
		pingPong.Options = opt.Options
	}
	cmd := exec.Command(pingPong.BinPath(), pingPong.BinArgs()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = pingPong.sysProcAttr()
	for _, opt := range opts {
		cmd.Env = opt.Env
	}
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _, _ = cmd.Process.Kill(), cmd.Wait() })
	return cmd, pingPong
}

type PingPong struct {
	Options

	binPath  string
	basePath string
	ping     Listener
	pong     Pipe
}

func New(t *testing.T) *PingPong {
	exe, err := os.Executable()
	require.NoError(t, err)

	basePath := newBasePath(t)
	ping := Pipe(filepath.Join(basePath, "ping"))
	pong := Pipe(filepath.Join(basePath, "pong"))

	listener, err := ping.Listen()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, listener.Close()) })

	return &PingPong{
		binPath:  exe,
		basePath: basePath,
		ping:     listener,
		pong:     pong,
	}
}

func (pp *PingPong) BinPath() string {
	return pp.binPath
}

func (pp *PingPong) BinArgs() (args []string) {
	ignoreGracefulTerminationRequests := "0"
	if pp.IgnoreGracefulTerminationRequests {
		ignoreGracefulTerminationRequests = "1"
	}

	return append(args, pingPongMarker, pp.basePath, ignoreGracefulTerminationRequests)
}

func (pp *PingPong) AwaitPing() error {
	ping, err := pp.ping.Accept()
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, ping)
	return errors.Join(err, ping.Close())
}

func (pp *PingPong) SendPong() error {
	out, err := pp.pong.OpenWriter()
	if err != nil {
		return err
	}
	_, err = out.Write([]byte("pong"))
	return errors.Join(err, out.Close())
}

type Pipe string

type Listener interface {
	io.Closer
	Accept() (io.ReadCloser, error)
}

func pingPongHook() {
	if len(os.Args) != 4 || os.Args[1] != pingPongMarker {
		return
	}

	basePath := os.Args[2]
	ignoreGracefulTerminationRequests := os.Args[3] == "1"

	exit(runHook(ignoreGracefulTerminationRequests, func() error {
		return runPingPong(basePath)
	}))
}

func runPingPong(basePath string) (err error) {
	ping := Pipe(filepath.Join(basePath, "ping"))

	pong, err := Pipe(filepath.Join(basePath, "pong")).Listen()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pong.Close()) }()

	if _, err := fmt.Fprintf(os.Stderr, "Sending ping ...\n"); err != nil {
		return err
	}
	out, err := ping.OpenWriter()
	if err != nil {
		return err
	}

	_, err = out.Write([]byte("ping"))
	err = errors.Join(err, out.Close())
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(os.Stderr, "Awaiting pong ...\n"); err != nil {
		return err
	}
	accepted, err := pong.Accept()
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, accepted)
	return errors.Join(err, accepted.Close())
}
