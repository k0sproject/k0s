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
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestLoadRuntimeConfig_Legacy(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "runtime-config")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// prepare k0sVars
	k0sVars := &CfgVars{
		DataDir:           "/var/lib/k0s-custom",
		RuntimeConfigPath: tmpfile.Name(),
	}

	content := []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  api:
    address: 10.2.3.4
`)
	err = os.WriteFile(k0sVars.RuntimeConfigPath, content, 0644)
	assert.NoError(t, err)

	spec, err := LoadRuntimeConfig(k0sVars)
	assert.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "/var/lib/k0s-custom", spec.K0sVars.DataDir)
	assert.Equal(t, os.Getpid(), spec.Pid)
	assert.NotNil(t, spec.NodeConfig)
	assert.NotNil(t, spec.NodeConfig.Spec.API)
	assert.Equal(t, "10.2.3.4", spec.NodeConfig.Spec.API.Address)
}

func TestLoadRuntimeConfig_K0sNotRunning(t *testing.T) {
	// create a temporary file for runtime config
	tmpfile, err := os.CreateTemp("", "runtime-config")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// prepare k0sVars
	k0sVars := &CfgVars{
		RuntimeConfigPath: tmpfile.Name(),
	}

	// write some content to the runtime config file
	content := []byte(`---
apiVersion: k0s.k0sproject.io/v1beta1
kind: RuntimeConfig
spec:
  nodeConfig:
    metadata:
      name: k0s
  pid: 9999999
`)
	err = os.WriteFile(k0sVars.RuntimeConfigPath, content, 0644)
	assert.NoError(t, err)

	// try to load runtime config and check if it returns an error
	spec, err := LoadRuntimeConfig(k0sVars)
	assert.Nil(t, spec)
	assert.ErrorIs(t, err, ErrK0sNotRunning)
}

func TestNewRuntimeConfig(t *testing.T) {
	// create a temporary directory for k0s files
	tempDir, err := os.MkdirTemp("", "k0s")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// create a temporary file for the runtime config
	tmpfile, err := os.CreateTemp("", "runtime-config")
	assert.NoError(t, err)
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// prepare k0sVars
	k0sVars := &CfgVars{
		RuntimeConfigPath: tmpfile.Name(),
		DataDir:           tempDir,
		nodeConfig: &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				API: &v1beta1.APISpec{Address: "10.0.0.1"},
			},
		},
	}

	// create a new runtime config and check if it's valid
	spec, err := NewRuntimeConfig(k0sVars)
	assert.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, tempDir, spec.K0sVars.DataDir)
	assert.Equal(t, os.Getpid(), spec.Pid)
	assert.NotNil(t, spec.NodeConfig)
	cfg, err := spec.K0sVars.NodeConfig()
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.1", cfg.Spec.API.Address)

	// try to create a new runtime config when one is already active and check if it returns an error
	_, err = NewRuntimeConfig(k0sVars)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrK0sAlreadyRunning)
}
