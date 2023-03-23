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

package status

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/stretchr/testify/require"
)

type mockProber struct{}

func (m *mockProber) State(maxCount int) prober.State {
	return prober.State{}
}

func TestStatusSocket(t *testing.T) {
	runDir := t.TempDir()

	var socket string
	if runtime.GOOS == "windows" {
		socket = `\\.\pipe\k0s-status` + strconv.Itoa(os.Getpid())
	} else {
		socket = filepath.Join(runDir, "k0s-status.sock")
	}

	component := &Status{
		Socket:            socket,
		Prober:            &mockProber{},
		StatusInformation: K0sStatus{Version: "status-socket-test", K0sVars: constant.CfgVars{RunDir: runDir}},
	}

	require.NoError(t, component.Init(context.Background()))
	component.L.Logger.SetOutput(io.Discard)
	require.NoError(t, component.Start(context.Background()))
	status, err := GetStatusInfo(socket)
	require.NoError(t, err)
	require.Equal(t, status.Version, "status-socket-test")
	require.NoError(t, component.Stop())
}
