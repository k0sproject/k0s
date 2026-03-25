// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mesosphere/toml-merge/pkg/patch"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
)

// Resolved and merged containerd configuration data.
type resolvedConfig struct {
	// Serialized configuration including merged CRI plugin configuration data.
	CRIConfig string

	// Paths to additional partial configuration files to be imported. Those
	// files won't contain any CRI plugin configuration data.
	ImportPaths []string
}

type configurer struct {
	loadPath   string
	pauseImage string

	log *logrus.Entry
}

// Resolves partial containerd configuration files from the import glob path. If
// a file contains a CRI plugin configuration section, it will be merged into
// k0s's default configuration, if not, it will be added to the list of import
// paths.
func (c *configurer) handleImports() (*resolvedConfig, error) {
	var importPaths []string

	defaultConfig, err := generateDefaultCRIConfig(c.pauseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate containerd default CRI config: %w", err)
	}

	filePaths, err := filepath.Glob(c.loadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to look for containerd import files: %w", err)
	}
	c.log.Debugf("found containerd config files to import: %v", filePaths)

	// Since the default config contains configuration data for the CRI plugin,
	// and containerd has decided to replace rather than merge individual plugin
	// configuration sections from imported config files, we need to manually
	// take care of merging k0s's defaults with the user overrides. Loop through
	// all import files and check if they contain any CRI plugin config. If they
	// do, treat them as merge patches to the default config, if they don't,
	// just add them as normal imports to be handled by containerd.
	finalConfig := string(defaultConfig)
	for _, filePath := range filePaths {
		c.log.Debugf("Processing containerd configuration file %s", filePath)

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		hasCRI, err := hasCRIPluginConfig(data)
		if err != nil {
			return nil, fmt.Errorf("failed to check for CRI plugin configuration in %s: %w", filePath, err)
		}

		if hasCRI {
			c.log.Infof("Found CRI plugin configuration in %s, treating as merge patch", filePath)
			finalConfig, err = patch.TOMLString(finalConfig, patch.FilePatches(filePath))
			if err != nil {
				return nil, fmt.Errorf("failed to merge data from %s into containerd configuration: %w", filePath, err)
			}
		} else {
			c.log.Debugf("No CRI plugin configuration found in %s, adding as-is to imports", filePath)
			importPaths = append(importPaths, filePath)
		}
	}

	return &resolvedConfig{CRIConfig: finalConfig, ImportPaths: importPaths}, nil
}

// Returns the default containerd config, including only the CRI plugin
// configuration, using the given image for sandbox containers. Uses the
// containerd package to generate all the rest, so this will be in sync with
// containerd's defaults for the CRI plugin.
func generateDefaultCRIConfig(sandboxContainerImage string) ([]byte, error) {
	// https://github.com/containerd/containerd/blob/main/docs/cri/config.md#full-configuration

	imagesConf := map[string]any{
		"pinned_images": map[string]any{
			"sandbox": sandboxContainerImage,
		},
	}

	runtimeConf := map[string]any{}

	if runtime.GOOS == "windows" {
		runtimeConf["cni"] = map[string]any{
			"conf_dir": `c:\etc\cni\net.d`,
			"bin_dirs": []any{`c:\opt\cni\bin`},
		}
	}

	return toml.Marshal(map[string]any{
		"version": 3,
		"plugins": map[string]any{
			"io.containerd.cri.v1.images":  imagesConf,
			"io.containerd.cri.v1.runtime": runtimeConf,
		},
	})
}

func hasCRIPluginConfig(data []byte) (bool, error) {
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return tree.HasPath([]string{"plugins", "io.containerd.cri.v1.runtime"}) || tree.HasPath([]string{"plugins", "io.containerd.cri.v1.images"}), nil
}
