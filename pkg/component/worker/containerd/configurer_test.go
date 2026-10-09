// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// On Linux, containerd-shim-runc-v2 derives runc's own state root from a
// per-runtime-class config option, not from containerd's --state flag. Left
// unset, it falls back to a hardcoded /run/containerd/runc, a path k0s does
// not own and `k0s reset` never cleans (#8285). k0s must point it at its own
// run dir instead.
func TestMarshalContainerdConfig_runcStateRoot(t *testing.T) {
	runDir := filepath.Join(string(filepath.Separator), "run", "k0s")

	data, err := marshalContainerdConfig("/imports", "pause:latest", runDir)
	require.NoError(t, err)

	tree, err := toml.LoadBytes(data)
	require.NoError(t, err)

	rootPath := []string{
		"plugins", "io.containerd.cri.v1.runtime", "containerd",
		"runtimes", "runc", "options", "Root",
	}

	if runtime.GOOS == "windows" {
		assert.False(t, tree.HasPath(rootPath),
			"Windows uses containerd-shim-runhcs-v1, not runc; no runc options should be set")
		return
	}

	require.True(t, tree.HasPath(rootPath), "expected a runc options.Root to be set")
	assert.Equal(t, filepath.Join(runDir, "containerd", "runc"), tree.GetPath(rootPath))
}

func TestMarshalContainerdConfig_isValid(t *testing.T) {
	data, err := marshalContainerdConfig("/imports", "pause:latest", "/run/k0s")
	require.NoError(t, err)
	assert.NoError(t, ValidateConfigFile(data))
}
