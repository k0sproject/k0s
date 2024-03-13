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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mesosphere/toml-merge/pkg/patch"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"

	criconfig "github.com/containerd/containerd/pkg/cri/config"
)

type configurer struct {
	loadPath       string
	pauseImage     string
	criRuntimePath string

	log *logrus.Entry
}

// Resolves containerd imports from the import glob path.
// If the partial config has CRI plugin enabled, it will add to the runc CRI config (single file).
// if no CRI plugin is found, it will add the file as-is to imports list returned.
// Once all files are processed the concatenated CRI config file is written and added to the imports list.
func (c *configurer) handleImports() ([]string, error) {
	var imports []string

	defaultConfig, err := generateDefaultCRIConfig(c.pauseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate containerd default CRI config: %w", err)
	}

	files, err := filepath.Glob(c.loadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to look for containerd import files: %w", err)
	}
	c.log.Debugf("found containerd config files to import: %v", files)

	// Since the default config contains configuration data for the CRI plugin,
	// and containerd has decided to replace rather than merge individual plugin
	// configuration sections from imported config files, we need to manually
	// take care of merging k0s's defaults with the user overrides. Loop through
	// all import files and check if they contain any CRI plugin config. If they
	// do, treat them as merge patches to the default config, if they don't,
	// just add them as normal imports to be handled by containerd.
	finalConfig := string(defaultConfig)
	for _, file := range files {
		c.log.Debugf("Processing containerd configuration file %s", file)

		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}

		hasCRI, err := hasCRIPluginConfig(data)
		if err != nil {
			return nil, fmt.Errorf("failed to check for CRI plugin configuration in %s: %w", file, err)
		}

		if hasCRI {
			c.log.Infof("Found CRI plugin configuration in %s, treating as merge patch", file)
			finalConfig, err = patch.TOMLString(finalConfig, patch.FilePatches(file))
			if err != nil {
				return nil, fmt.Errorf("failed to merge data from %s into containerd configuration: %w", file, err)
			}
		} else {
			c.log.Debugf("No CRI plugin configuration found in %s, adding as-is to imports", file)
			imports = append(imports, file)
		}
	}

	// Write the CRI config to a file and add it to imports
	err = os.WriteFile(c.criRuntimePath, []byte(finalConfig), 0644)
	if err != nil {
		return nil, err
	}
	imports = append(imports, escapedPath(c.criRuntimePath))

	return imports, nil
}

func escapedPath(s string) string {
	// double escape for windows because containerd expects
	// double backslash in the configuration but golang templates
	// unescape double slash to a single slash
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(s, "\\", "\\\\")
	}
	return s
}

// Returns the default containerd config, including only the CRI plugin
// configuration, using the given image for sandbox containers. Uses the
// containerd package to generate all the rest, so this will be in sync with
// containerd's defaults for the CRI plugin.
func generateDefaultCRIConfig(sandboxContainerImage string) ([]byte, error) {
	criPluginConfig := criconfig.DefaultConfig()
	// Set pause image
	criPluginConfig.SandboxImage = sandboxContainerImage
	if runtime.GOOS == "windows" {
		criPluginConfig.CniConfig.NetworkPluginBinDir = "c:\\opt\\cni\\bin"
		criPluginConfig.CniConfig.NetworkPluginConfDir = "c:\\opt\\cni\\conf"
	}
	// We need to use custom struct so we can unmarshal the CRI plugin config only
	containerdConfig := struct {
		Version int
		Plugins map[string]interface{} `toml:"plugins"`
	}{
		Version: 2,
		Plugins: map[string]interface{}{
			"io.containerd.grpc.v1.cri": criPluginConfig,
		},
	}

	return toml.Marshal(containerdConfig)
}

func hasCRIPluginConfig(data []byte) (bool, error) {
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse TOML: %w", err)
	}

	return tree.HasPath([]string{"plugins", "io.containerd.grpc.v1.cri"}), nil
}
