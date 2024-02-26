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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mesosphere/toml-merge/pkg/patch"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"

	criconfig "github.com/containerd/containerd/pkg/cri/config"
)

type CRIConfigurer struct {
	loadPath       string
	pauseImage     string
	criRuntimePath string

	log *logrus.Entry
}

// HandleImports Resolves containerd imports from the import glob path.
// If the partial config has CRI plugin enabled, it will add to the runc CRI config (single file).
// if no CRI plugin is found, it will add the file as-is to imports list returned.
// Once all files are processed the concatenated CRI config file is written and added to the imports list.
func (c *CRIConfigurer) HandleImports() ([]string, error) {
	var imports []string
	var criConfigBuffer bytes.Buffer

	// Add default runc based CRI config
	err := c.generateDefaultCRIConfig(&criConfigBuffer)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(c.loadPath)
	c.log.Debugf("found containerd config files: %v", files)
	if err != nil {
		return nil, err
	}

	finalConfig := criConfigBuffer.String()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		c.log.Debugf("parsing containerd config file: %s", file)
		hasCRI, err := c.hasCRIPluginConfig(data)
		if err != nil {
			return nil, err
		}
		if hasCRI {
			c.log.Infof("found CRI plugin config in %s, merging to CRI config", file)
			// Merge to the existing CRI config
			finalConfig, err = patch.TOMLString(finalConfig, patch.FilePatches(file))
			if err != nil {
				return nil, fmt.Errorf("failed to merge CRI config from %s: %w", file, err)
			}
		} else {
			c.log.Debugf("adding %s as-is to imports", file)
			// Add file to imports
			imports = append(imports, escapedPath(file))
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

// generateDefaultCRIConfig generates the default CRI config and writes it to the given writer
// It uses the containerd containerd package to generate the config so we can keep it in sync with containerd
func (c *CRIConfigurer) generateDefaultCRIConfig(w io.Writer) error {
	criPluginConfig := criconfig.DefaultConfig()
	// Set pause image
	criPluginConfig.SandboxImage = c.pauseImage
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

	err := toml.NewEncoder(w).Encode(containerdConfig)
	if err != nil {
		return fmt.Errorf("failed to generate containerd default CRI config: %w", err)
	}
	return nil
}

func (c *CRIConfigurer) hasCRIPluginConfig(data []byte) (bool, error) {
	var tomlConfig map[string]interface{}
	if err := toml.Unmarshal(data, &tomlConfig); err != nil {
		return false, err
	}
	c.log.Debugf("parsed containerd config: %+v", tomlConfig)
	if _, ok := tomlConfig["plugins"]; ok {
		if _, ok := tomlConfig["plugins"].(map[string]interface{})["io.containerd.grpc.v1.cri"]; ok {
			return true, nil
		}
	}
	return false, nil
}
