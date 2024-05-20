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
	_ "embed"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed pingpong.sh
var script []byte

type PingPong struct {
	IgnoreGracefulShutdownRequest bool // If set, SIGTERM won't terminate the program.

	shellPath, pipe, script string
}

func New(t *testing.T) *PingPong {
	shellPath, err := exec.LookPath("sh")
	require.NoError(t, err)

	tmpDir := t.TempDir()
	pp := PingPong{
		shellPath: shellPath,
		pipe:      filepath.Join(tmpDir, "pingpong"),
		script:    filepath.Join(tmpDir, "pingpong.sh"),
	}

	err = syscall.Mkfifo(pp.pipe, 0600)
	require.NoError(t, err, "mkfifo failed for %s", pp.pipe)
	err = os.WriteFile(pp.script, script, 0700)
	require.NoError(t, err, "Failed to write script file")
	return &pp
}

func (pp *PingPong) BinPath() string {
	return pp.shellPath
}

func (pp *PingPong) BinArgs() []string {
	var ignoreSIGTERM string
	if pp.IgnoreGracefulShutdownRequest {
		ignoreSIGTERM = "1"
	}

	return []string{pp.script, pp.pipe, ignoreSIGTERM}
}

func (pp *PingPong) AwaitPing() (err error) {
	// The open for reading call will block until the
	// script tries to open the file for writing.
	f, err := os.OpenFile(pp.pipe, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, f)
	return errors.Join(err, f.Close())
}

func (pp *PingPong) SendPong() error {
	// The open for writing call will block until the
	// script tries to open the file for reading.
	f, err := os.OpenFile(pp.pipe, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString("pong\n")
	return errors.Join(err, f.Close())
}
