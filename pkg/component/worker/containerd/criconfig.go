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

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"

	criconfig "github.com/containerd/containerd/pkg/cri/config"
)

const importsPath = "/etc/k0s/containerd.d/*.toml"
const containerdCRIConfigPath = "/run/k0s/containerd-cri.toml"

type CRIConfigurer struct {
	loadPath       string
	pauseImage     string
	criRuntimePath string

	log *logrus.Entry
}

func NewConfigurer() *CRIConfigurer {

	pauseImage := v1beta1.ImageSpec{
		Image:   constant.KubePauseContainerImage,
		Version: constant.KubePauseContainerImageVersion,
	}
	return &CRIConfigurer{
		loadPath:       importsPath,
		criRuntimePath: containerdCRIConfigPath,
		pauseImage:     pauseImage.URI(),
		log:            logrus.WithField("component", "containerd"),
	}
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
			c.log.Debugf("found CRI plugin config in %s, appending to CRI config", file)
			// Append all CRI plugin configs into single file
			// and add it to imports
			criConfigBuffer.WriteString(fmt.Sprintf("# appended from %s\n", file))
			_, err := criConfigBuffer.Write(data)
			if err != nil {
				return nil, err
			}
		} else {
			c.log.Debugf("adding %s as-is to imports", file)
			// Add file to imports
			imports = append(imports, file)
		}
	}

	// Write the CRI config to a file and add it to imports
	err = os.WriteFile(c.criRuntimePath, criConfigBuffer.Bytes(), 0644)
	if err != nil {
		return nil, err
	}
	imports = append(imports, c.criRuntimePath)

	return imports, nil
}

// We need to use custom struct so we can unmarshal the CRI plugin config only
type config struct {
	Version int
	Plugins map[string]interface{} `toml:"plugins"`
}

// generateDefaultCRIConfig generates the default CRI config and writes it to the given writer
// It uses the containerd containerd package to generate the config so we can keep it in sync with containerd
func (c *CRIConfigurer) generateDefaultCRIConfig(w io.Writer) error {
	criPluginConfig := criconfig.DefaultConfig()
	// Set pause image
	criPluginConfig.SandboxImage = c.pauseImage

	containerdConfig := config{
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
