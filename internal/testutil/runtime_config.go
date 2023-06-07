/*
Copyright 2021 k0s authors

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

package testutil

import (
	"os"
	"path"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
)

type ConfigGetter struct {
	NodeConfig bool
	YamlData   string

	t       *testing.T
	k0sVars *config.CfgVars
}

// NewConfigGetter sets the parameters required to fetch a fake config for testing
func NewConfigGetter(t *testing.T, yamlData string, isNodeConfig bool, k0sVars *config.CfgVars) *ConfigGetter {
	return &ConfigGetter{
		YamlData:   yamlData,
		NodeConfig: isNodeConfig,
		t:          t,
		k0sVars:    k0sVars,
	}
}

// FakeRuntimeConfig takes a yaml construct and returns a config object from a fake runtime config path
func (c *ConfigGetter) FakeConfigFromFile() *v1beta1.ClusterConfig {
	c.k0sVars.RuntimeConfigPath = c.initRuntimeConfig()
	rtc, err := config.LoadRuntimeConfig(c.k0sVars)
	require.NoError(c.t, err, "failed to create fake runtime config")
	defer require.NoError(c.t, os.Remove(c.k0sVars.RuntimeConfigPath))

	return rtc.NodeConfig
}

func (c *ConfigGetter) initRuntimeConfig() string {
	vars := c.k0sVars.DeepCopy()
	cfg, err := v1beta1.ConfigFromString(c.YamlData, c.getStorageSpec())
	require.NoError(c.t, err, "failed to parse config")
	vars.SetNodeConfig(cfg)
	vars.RuntimeConfigPath = path.Join(c.t.TempDir(), "fake-k0s-runtime.yaml")
	_, err = config.NewRuntimeConfig(vars)
	require.NoError(c.t, err, "failed to create fake runtime config")

	return vars.RuntimeConfigPath
}

func (c *ConfigGetter) getStorageSpec() *v1beta1.StorageSpec {
	var storage *v1beta1.StorageSpec

	if c.k0sVars.DefaultStorageType == "kine" {
		storage = &v1beta1.StorageSpec{
			Type: v1beta1.KineStorageType,
			Kine: v1beta1.DefaultKineConfig(c.k0sVars.DataDir),
		}
	}
	return storage
}
