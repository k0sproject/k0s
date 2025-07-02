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
	"fmt"
	"io"
	"net"
	"path/filepath"
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
