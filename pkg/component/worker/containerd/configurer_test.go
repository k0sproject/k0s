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

	serverconfig "github.com/containerd/containerd/services/server/config"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurer_HandleImports(t *testing.T) {
	t.Run("should merge configuration files containing CRI plugin configuration sections", func(t *testing.T) {
		importsPath := t.TempDir()
		criRuntimeConfig := `
[plugins]
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "zfs"
`
		err := os.WriteFile(filepath.Join(importsPath, "foo.toml"), []byte(criRuntimeConfig), 0644)
		require.NoError(t, err)
		c := configurer{
			loadPath:   filepath.Join(importsPath, "*.toml"),
			pauseImage: "pause:42",
			log:        logrus.New().WithField("test", t.Name()),
		}
		criConfig, err := c.handleImports()
		assert.NoError(t, err)
		require.NotNil(t, criConfig)
		assert.Empty(t, criConfig.ImportPaths, "files containing CRI plugin configuration sections should be merged, not imported")

		// Dump the config for inspection
		t.Logf("CRI config:\n%s", criConfig.CRIConfig)

		criConfigPath := filepath.Join(t.TempDir(), "cri.toml")
		require.NoError(t, os.WriteFile(criConfigPath, []byte(criConfig.CRIConfig), 0644))

		// Load the criRuntimeConfig and verify the settings are correct
		var containerdConfig serverconfig.Config
		require.NoError(t, serverconfig.LoadConfig(criConfigPath, &containerdConfig))

		assert.Equal(t, 2, containerdConfig.Version)
		criPluginConfig := containerdConfig.Plugins["io.containerd.grpc.v1.cri"]
		require.NotNil(t, criPluginConfig, "No CRI plugin configuration section found")
		sandboxImage := criPluginConfig.Get("sandbox_image")
		assert.Equal(t, "pause:42", sandboxImage, "Custom pause image not found in CRI configuration")
		snapshotter := criPluginConfig.GetPath([]string{"containerd", "snapshotter"})
		assert.Equal(t, "zfs", snapshotter, "Overridden snapshotter not found in CRI configuration")
	})

	t.Run("should have no imports if imports dir is empty", func(t *testing.T) {
		c := configurer{
			loadPath: filepath.Join(t.TempDir(), "*.toml"),
			log:      logrus.New().WithField("test", t.Name()),
		}
		criConfig, err := c.handleImports()
		assert.NoError(t, err)
		assert.Empty(t, criConfig.ImportPaths)
	})

	t.Run("should import configuration files not containing a CRI plugin configuration section", func(t *testing.T) {
		importsPath := t.TempDir()
		criRuntimeConfig := `
foo = "bar"
version = 2
`
		nonCriConfigPath := filepath.Join(importsPath, "foo.toml")
		err := os.WriteFile(nonCriConfigPath, []byte(criRuntimeConfig), 0644)
		require.NoError(t, err)
		c := configurer{
			loadPath: filepath.Join(importsPath, "*.toml"),
			log:      logrus.New().WithField("test", t.Name()),
		}
		criConfig, err := c.handleImports()
		assert.NoError(t, err)
		assert.Equal(t, []string{nonCriConfigPath}, criConfig.ImportPaths)
	})
}
