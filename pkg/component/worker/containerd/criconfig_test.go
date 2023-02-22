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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestCRIConfigurer_hasCRIPluginConfig(t *testing.T) {
	t.Run("should return true if config has cri plugin configs", func(t *testing.T) {
		cfg := `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
    endpoint = ["https://registry-1.docker.io"]
`
		c := NewConfigurer()
		hasCRIPluginConfig, err := c.hasCRIPluginConfig([]byte(cfg))
		require.NoError(t, err)
		require.True(t, hasCRIPluginConfig)
	})

	t.Run("should return false if config has no cri plugin configs", func(t *testing.T) {
		cfg := `
timeout = 3
version = 2
`
		c := NewConfigurer()
		hasCRIPluginConfig, err := c.hasCRIPluginConfig([]byte(cfg))
		require.NoError(t, err)
		require.False(t, hasCRIPluginConfig)
	})

}

func TestCRIConfigurer_HandleImports(t *testing.T) {
	t.Run("should have single import for CRI if there's nothing in imports dir", func(t *testing.T) {
		testLoadPath := filepath.Join(t.TempDir(), "*.toml")
		criRuntimePath := filepath.Join(t.TempDir(), "cri.toml")
		c := CRIConfigurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.HandleImports()
		require.NoError(t, err)
		require.Len(t, imports, 1)
		require.Equal(t, criRuntimePath, imports[0])
	})

	t.Run("should have single import for all CRI configs", func(t *testing.T) {
		tmp := t.TempDir()
		testLoadPath := filepath.Join(tmp, "*.toml")
		criRuntimePath := filepath.Join(t.TempDir(), "cri.toml")
		criRuntimeConfig := `
[plugins."io.containerd.grpc.v1.cri".registry.mirrors]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
`
		err := os.WriteFile(filepath.Join(tmp, "foo.toml"), []byte(criRuntimeConfig), 0644)
		require.NoError(t, err)
		c := CRIConfigurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.HandleImports()
		require.NoError(t, err)
		require.Len(t, imports, 1)
		require.Contains(t, imports, criRuntimePath)
		criCfg := loadFile(t, criRuntimePath)
		require.Contains(t, criCfg, criRuntimeConfig)
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
		c := CRIConfigurer{
			loadPath:       testLoadPath,
			criRuntimePath: criRuntimePath,
			log:            logrus.New().WithField("test", t.Name()),
		}
		imports, err := c.HandleImports()
		require.NoError(t, err)
		require.Len(t, imports, 2)
		require.Contains(t, imports, criRuntimePath)
		require.Contains(t, imports, nonCriConfigPath)
	})
}

func loadFile(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
