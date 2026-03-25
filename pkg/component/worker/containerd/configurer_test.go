// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	serverconfig "github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurer_HandleImports(t *testing.T) {
	t.Run("should merge configuration files containing CRI plugin configuration sections", func(t *testing.T) {
		importsPath := t.TempDir()
		criRuntimeConfig := `
version = 3
[plugins]
  [plugins."io.containerd.cri.v1.images"]
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
		require.NoError(t, serverconfig.LoadConfig(context.Background(), criConfigPath, &containerdConfig))

		assert.Equal(t, 3, containerdConfig.Version)

		imagesConf := containerdConfig.Plugins["io.containerd.cri.v1.images"]
		require.NotNil(t, imagesConf, "No CRI images plugin configuration section found")

		imagesConfTree, _ := toml.TreeFromMap(imagesConf.(map[string]any))

		sandboxImage := imagesConfTree.GetPath([]string{"pinned_images", "sandbox"})
		assert.Equal(t, "pause:42", sandboxImage, "Custom pause image not found in CRI configuration")
		snapshotter := imagesConfTree.GetPath([]string{"snapshotter"})
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
version = 3
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
