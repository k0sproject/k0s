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

	srvconfig "github.com/containerd/containerd/services/server/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestConfigurer_HandleImports(t *testing.T) {
	t.Run("should merge CRI configs", func(t *testing.T) {
		tmp := t.TempDir()
		testLoadPath := filepath.Join(tmp, "*.toml")
		criRuntimePath := filepath.Join(t.TempDir(), "cri.toml")
		criRuntimeConfig := `
[plugins]
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "zfs"
`
		err := os.WriteFile(filepath.Join(tmp, "foo.toml"), []byte(criRuntimeConfig), 0644)
		require.NoError(t, err)
		c := configurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.handleImports()
		require.NoError(t, err)
		require.Len(t, imports, 1)
		require.Contains(t, imports, escapedPath(criRuntimePath))

		// Dump the config for inspection
		b, _ := os.ReadFile(criRuntimePath)
		t.Logf("cri config:\n%s", string(b))

		// Load the criRuntimeConfig and verify the settings are correct
		containerdConfig := &srvconfig.Config{}
		err = srvconfig.LoadConfig(criRuntimePath, containerdConfig)
		require.NoError(t, err)

		criConfig := containerdConfig.Plugins["io.containerd.grpc.v1.cri"]
		snapshotter := criConfig.GetPath([]string{"containerd", "snapshotter"})
		require.Equal(t, "zfs", snapshotter)
	})

	t.Run("should have single import for CRI if there's nothing in imports dir", func(t *testing.T) {
		testLoadPath := filepath.Join(t.TempDir(), "*.toml")
		criRuntimePath := filepath.Join(t.TempDir(), "cri.toml")
		c := configurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.handleImports()
		require.NoError(t, err)
		require.Len(t, imports, 1)
		require.Equal(t, escapedPath(criRuntimePath), imports[0])
	})

	t.Run("should have two imports when one non CRI snippet", func(t *testing.T) {
		tmp := t.TempDir()
		testLoadPath := filepath.Join(tmp, "*.toml")
		criRuntimePath := filepath.Join(t.TempDir(), "cri.toml")
		criRuntimeConfig := `
foo = "bar"
version = 2
`
		nonCriConfigPath := filepath.Join(tmp, "foo.toml")
		err := os.WriteFile(nonCriConfigPath, []byte(criRuntimeConfig), 0644)
		require.NoError(t, err)
		c := configurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.handleImports()
		require.NoError(t, err)
		require.Len(t, imports, 2)
		require.Contains(t, imports, escapedPath(criRuntimePath))
		require.Contains(t, imports, escapedPath(nonCriConfigPath))
	})
}
