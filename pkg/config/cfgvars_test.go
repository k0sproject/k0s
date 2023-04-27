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
	"reflect"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

type FakeFlagSet struct {
	values map[string]any
}

type FakeCommand struct {
	flagSet *FakeFlagSet
}

func (f *FakeCommand) Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("fake", pflag.ContinueOnError)
	for k, val := range f.flagSet.values {
		switch v := val.(type) {
		case bool:
			fs.Bool(k, v, "")
		case string:
			fs.String(k, v, "")
		case int:
			fs.Int(k, v, "")
		}
	}
	return fs
}

func TestWithCommand(t *testing.T) {
	// Create a fake flag set with some values
	fakeFlags := &FakeFlagSet{
		values: map[string]any{
			"data-dir":              "/path/to/data",
			"config":                "/path/to/config",
			"status-socket":         "/path/to/socket",
			"enable-dynamic-config": true,
		},
	}

	// Create a fake command that returns the fake flag set
	fakeCmd := &FakeCommand{
		flagSet: fakeFlags,
	}

	// Create a CfgVars struct and apply the options
	c := &CfgVars{}
	WithCommand(fakeCmd)(c)

	assert.Equal(t, "/path/to/data", c.DataDir)
	assert.Equal(t, "/path/to/config", c.StartupConfigPath)
	assert.Equal(t, "/path/to/socket", c.StatusSocketPath)
	assert.True(t, c.EnableDynamicConfig)
}

func TestWithCommand_DefaultsAndOverrides(t *testing.T) {
	// Define test cases for the single flag
	testCases := []struct {
		name                string
		flagValue           any
		expectedStorageType string
	}{
		{
			name:                "single flag is set to false",
			flagValue:           false,
			expectedStorageType: v1beta1.EtcdStorageType,
		},
		{
			name:                "single flag is set to true",
			flagValue:           true,
			expectedStorageType: v1beta1.KineStorageType,
		},
	}

	// Create a fake command with a flag set that includes the test cases
	fakeFlags := &FakeFlagSet{
		values: map[string]any{},
	}
	for _, tc := range testCases {
		fakeFlags.values["single"] = tc.flagValue

		c := &CfgVars{}
		WithCommand(&FakeCommand{flagSet: fakeFlags})(c)

		assert.Equal(t, tc.expectedStorageType, c.DefaultStorageType, tc.name)
	}
}

func TestNewCfgVars_DataDir(t *testing.T) {
	tests := []struct {
		name     string
		fakeCmd  command
		dirs     []string
		expected *CfgVars
	}{
		{
			name:     "default data dir",
			fakeCmd:  &FakeCommand{flagSet: &FakeFlagSet{}},
			expected: &CfgVars{DataDir: constant.DataDirDefault},
		},
		{
			name: "custom data dir",
			fakeCmd: &FakeCommand{
				flagSet: &FakeFlagSet{values: map[string]any{"data-dir": "/path/to/data"}},
			},
			expected: &CfgVars{DataDir: "/path/to/data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCfgVars(tt.fakeCmd, tt.dirs...)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.DataDir, c.DataDir)
		})
	}
}

func TestNodeConfig_Default(t *testing.T) {
	oldDefaultPath := defaultConfigPath
	defer func() { defaultConfigPath = oldDefaultPath }()
	defaultConfigPath = "/tmp/k0s.yaml.nonexistent"

	c := &CfgVars{StartupConfigPath: defaultConfigPath}

	nodeConfig, err := c.NodeConfig()

	assert.NoError(t, err)
	assert.NotNil(t, nodeConfig)
	assert.True(t, reflect.DeepEqual(v1beta1.DefaultClusterConfig(c.defaultStorageSpec()), nodeConfig))
}
