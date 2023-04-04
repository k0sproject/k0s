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

package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isK0sManagedConfig(t *testing.T) {

	t.Run("should return true if file does not exist", func(t *testing.T) {
		isManaged, err := isK0sManagedConfig("non-existent.toml")
		require.NoError(t, err)
		require.True(t, isManaged)
	})

	t.Run("should return true if md5 matches with pre 1.27 default config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "containerd.toml")
		cfg := `
# This is a placeholder configuration for k0s managed containerD.
# If you wish to customize the config replace this file with your custom configuration.
# For reference see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
version = 2
`
		err := os.WriteFile(configPath, []byte(cfg), 0644)
		require.NoError(t, err)
		isManaged, err := isK0sManagedConfig(configPath)
		require.NoError(t, err)
		require.True(t, isManaged)
	})

	t.Run("should return false if md5 differs with pre 1.27 default config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "containerd.toml")
		cfg := `
# This is a placeholder configuration for k0s managed containerD.
# If you wish to customize the config replace this file with your custom configuration.
# For reference see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
version = 2
[plugins."io.containerd.grpc.v1.cri".registry.mirrors]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
    endpoint = ["https://registry-1.docker.io"]

`
		err := os.WriteFile(configPath, []byte(cfg), 0644)
		require.NoError(t, err)
		isManaged, err := isK0sManagedConfig(configPath)
		require.NoError(t, err)
		require.False(t, isManaged)
	})
}
