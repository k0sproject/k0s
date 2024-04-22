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

package containerd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
)

func Test_isK0sManagedConfig(t *testing.T) {

	t.Run("should return true if file does not exist", func(t *testing.T) {
		isManaged, err := isK0sManagedConfig(filepath.Join(t.TempDir(), "non-existent.toml"))
		require.NoError(t, err)
		require.True(t, isManaged)
	})

	t.Run("should return true for generated default config", func(t *testing.T) {
		defaultConfigPath := filepath.Join(t.TempDir(), "default.toml")

		underTest := Component{
			K0sVars: &config.CfgVars{
				RunDir: /* The merged config file will be written here: */ t.TempDir(),
			},
			confPath:/* The main config file will be written here: */ defaultConfigPath,
			importsPath:/* some empty dir: */ t.TempDir(),
			Profile:/* Some non-nil pause image: */ &workerconfig.Profile{PauseImage: &v1beta1.ImageSpec{}},
		}
		err := underTest.setupConfig()

		require.NoError(t, err)
		require.FileExists(t, defaultConfigPath, "The generated config file is missing.")

		isManaged, err := isK0sManagedConfig(defaultConfigPath)
		require.NoError(t, err)
		require.True(t, isManaged, "The generated config file should qualify as k0s-managed, but doesn't.")
	})

	t.Run("should return false if file has no marker", func(t *testing.T) {
		unmanagedPath := filepath.Join(t.TempDir(), "unmanaged.toml")
		require.NoError(t, os.WriteFile(unmanagedPath, []byte(" # k0s_managed=true"), 0644))

		isManaged, err := isK0sManagedConfig(unmanagedPath)
		require.NoError(t, err)
		require.False(t, isManaged)
	})

	t.Run("should return true for pre-1.30 generated config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "containerd.toml")
		cfg := `
# k0s_managed=true
# This is a placeholder configuration for k0s managed containerD. 
# If you wish to override the config, remove the first line and replace this file with your custom configuration.
# For reference see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
version = 2
imports = [
	"/run/k0s/containerd-cri.toml",
]
`
		err := os.WriteFile(configPath, []byte(cfg), 0644)
		require.NoError(t, err)
		isManaged, err := isK0sManagedConfig(configPath)
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
# This is a placeholder configuration for k0s managed containerd.
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
