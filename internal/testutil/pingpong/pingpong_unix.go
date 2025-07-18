//go:build unix

// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package pingpong

import (
	"io"
	"os"
	"syscall"
	"testing"
)

func newBasePath(t *testing.T) string {
	return t.TempDir()
}

func (p Pipe) OpenWriter() (io.WriteCloser, error) {
	// The open for writing call will block until the
	// script tries to open the file for reading.
	return os.OpenFile(string(p), os.O_WRONLY, 0)
}

func (p Pipe) Listen() (Listener, error) {
	if err := syscall.Mkfifo(string(p), 0600); err != nil {
		return nil, err
	}
	return &unixSocketListener{string(p)}, nil
}

// Implements [Listener] on UNIX systems.
type unixSocketListener struct {
	path string
}

func (f *unixSocketListener) Accept() (io.ReadCloser, error) {
	// The open for reading call will block until the
	// script tries to open the file for writing.
	return os.OpenFile(f.path, os.O_RDONLY, 0)
}

func (f *unixSocketListener) Close() error {
	return os.Remove(f.path)
}
