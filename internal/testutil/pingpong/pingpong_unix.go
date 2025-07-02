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
