//go:build unix

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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

type PingPong struct {
	shellPath, ping, pong string
}

func New(t *testing.T) *PingPong {
	shellPath, err := exec.LookPath("sh")
	require.NoError(t, err)

	tmpDir := t.TempDir()
	pp := PingPong{
		shellPath,
		filepath.Join(tmpDir, "pipe.ping"),
		filepath.Join(tmpDir, "pipe.pong"),
	}

	for _, path := range []string{pp.ping, pp.pong} {
		err := syscall.Mkfifo(path, 0600)
		require.NoError(t, err, "Mkfifo failed for %s", path)
	}

	return &pp
}

func (pp *PingPong) BinPath() string {
	return pp.shellPath
}

func (pp *PingPong) BinArgs() []string {
	return []string{"-euc", `cat -- "$1" && echo pong >"$2"`, "--", pp.ping, pp.pong}
}

func (pp *PingPong) AwaitPing() (err error) {
	f, err := os.OpenFile(pp.ping, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	// The write will block until the process reads from the FIFO file.
	if _, err := f.Write([]byte("ping\n")); err != nil {
		return err
	}

	return nil
}

func (pp *PingPong) SendPong() (err error) {
	// Read from the FIFO file to unblock the process.
	_, err = os.ReadFile(pp.pong)
	return err
}
