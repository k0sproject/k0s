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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestLoadRuntimeConfig(t *testing.T) {
	// write some content to the runtime config file
	rtConfigPath := filepath.Join(t.TempDir(), "runtime-config")
	content := []byte(`---
apiVersion: k0s.k0sproject.io/v1beta1
kind: RuntimeConfig
spec:
  nodeConfig:
    metadata:
      name: k0s
`)
	require.NoError(t, os.WriteFile(rtConfigPath, content, 0644))

	// try to load runtime config and check if it returns an error
	spec, err := LoadRuntimeConfig(rtConfigPath)
	assert.Nil(t, spec)
	assert.ErrorIs(t, err, ErrK0sNotRunning)
}

func TestNewRuntimeConfig(t *testing.T) {
	// Create regular configuration file
	tempDir := t.TempDir()
	startupConfigPath := filepath.Join(tempDir, "startup-config")
	startupConfig, err := yaml.Marshal(&v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{Address: "10.0.0.1"},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(startupConfigPath, startupConfig, 0644))

	rtConfigPath := filepath.Join(tempDir, "runtime-config")

	// prepare k0sVars
	k0sVars := &CfgVars{
		StartupConfigPath: startupConfigPath,
		RuntimeConfigPath: rtConfigPath,
		DataDir:           tempDir,
	}

	// Check if the node config can be loaded properly
	nodeConfig, err := k0sVars.NodeConfig()
	if assert.NoError(t, err) {
		assert.Equal(t, "10.0.0.1", nodeConfig.Spec.API.Address)
	}

	// create a new runtime config and check if it's valid
	cfg, err := NewRuntimeConfig(k0sVars, nodeConfig)
	if assert.NoError(t, err) && assert.NotNil(t, cfg) && assert.NotNil(t, cfg.Spec) {
		t.Cleanup(func() { assert.NoError(t, cfg.Spec.Cleanup()) })
		assert.Same(t, k0sVars, cfg.Spec.K0sVars)
		assert.Same(t, nodeConfig, cfg.Spec.NodeConfig)
	}
	assert.FileExists(t, rtConfigPath)

	// try to create a new runtime config when one is already active and check if it returns an error
	_, err = NewRuntimeConfig(k0sVars, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrK0sAlreadyRunning)
}
