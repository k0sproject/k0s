// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package pingpong

import (
	"fmt"
	"io"
	"net"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"

	"github.com/stretchr/testify/require"
)

func newBasePath(t *testing.T) string {
	guid, err := guid.NewV4()
	require.NoError(t, err)
	namespace := t.Name() + "_" + guid.String()
	return filepath.Join(`\\.\pipe`, namespace)
}

func (p Pipe) OpenWriter() (io.WriteCloser, error) {
	return winio.DialPipe(string(p), nil)
}

func (pp *PingPong) sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func (p Pipe) Listen() (Listener, error) {
	l, err := winio.ListenPipe(string(p), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}
	return &namedPipeListener{l}, nil
}

// Implements [Listener] on Windows systems.
type namedPipeListener struct {
	net.Listener
}

func (l *namedPipeListener) Accept() (io.ReadCloser, error) {
	return l.Listener.Accept()
}
