// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FakeFlagSet struct {
	values map[string]any
}

type FakeCommand struct {
	stdin   io.Reader
	flagSet *FakeFlagSet
}

func (f *FakeCommand) InOrStdin() io.Reader {
	return f.stdin
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
			"data-dir":            "/path/to/data",
			"kubelet-root-dir":    "/path/to/kubelet",
			"containerd-root-dir": "/path/to/containerd",
			"config":              "/path/to/config",
			"status-socket":       "/path/to/socket",
		},
	}

	// Create a fake command that returns the fake flag set
	in := bytes.NewReader(nil)
	fakeCmd := &FakeCommand{
		stdin:   in,
		flagSet: fakeFlags,
	}

	// Create a CfgVars struct and apply the options
	c := &CfgVars{}
	WithCommand(fakeCmd)(c)

	dir, err := filepath.Abs("/path/to/kubelet")
	require.NoError(t, err)
	cDir, err := filepath.Abs("/path/to/containerd")
	require.NoError(t, err)

	assert.Same(t, in, c.stdin)
	assert.Equal(t, "/path/to/data", c.DataDir)
	assert.Equal(t, dir, c.KubeletRootDir)
	assert.Equal(t, cDir, c.ContainerdRootDir)
	assert.Equal(t, "/path/to/config", c.StartupConfigPath)
	assert.Equal(t, "/path/to/socket", c.StatusSocketPath)
}

func TestWithCommand_DefaultsAndOverrides(t *testing.T) {
	// Define test cases for the single flag
	testCases := []struct {
		name                string
		flagValue           any
		expectedStorageType v1beta1.StorageType
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
			require.NoError(t, err)
			assert.Equal(t, tt.expected.DataDir, c.DataDir)
		})
	}
}

func TestNewCfgVars_KubeletRootDir(t *testing.T) {
	tests := []struct {
		name     string
		fakeCmd  command
		dirs     []string
		expected *CfgVars
	}{
		{
			name:     "default kubelet root dir",
			fakeCmd:  &FakeCommand{flagSet: &FakeFlagSet{}},
			expected: &CfgVars{KubeletRootDir: filepath.Join(constant.DataDirDefault, "kubelet")},
		},
		{
			name: "default kubelet root dir when datadir set",
			fakeCmd: &FakeCommand{
				flagSet: &FakeFlagSet{values: map[string]any{"data-dir": "/path/to/data"}},
			},
			expected: &CfgVars{KubeletRootDir: "/path/to/data/kubelet"},
		},
		{
			name: "custom kubelet root dir",
			fakeCmd: &FakeCommand{
				flagSet: &FakeFlagSet{values: map[string]any{"kubelet-root-dir": "/path/to/kubelet"}},
			},
			expected: &CfgVars{KubeletRootDir: "/path/to/kubelet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCfgVars(tt.fakeCmd, tt.dirs...)
			require.NoError(t, err)
			expected, err := filepath.Abs(tt.expected.KubeletRootDir)
			require.NoError(t, err)
			assert.Equal(t, expected, c.KubeletRootDir)
		})
	}
}

func TestNewCfgVars_ContainerdRootDir(t *testing.T) {
	tests := []struct {
		name     string
		fakeCmd  command
		dirs     []string
		expected *CfgVars
	}{
		{
			name:     "default containerd root dir",
			fakeCmd:  &FakeCommand{flagSet: &FakeFlagSet{}},
			expected: &CfgVars{ContainerdRootDir: filepath.Join(constant.DataDirDefault, "containerd")},
		},
		{
			name:     "default containerd root dir when datadir set",
			fakeCmd:  &FakeCommand{flagSet: &FakeFlagSet{values: map[string]any{"data-dir": "/path/to/data"}}},
			expected: &CfgVars{ContainerdRootDir: "/path/to/data/containerd"},
		},
		{
			name:     "custom containerd root dir",
			fakeCmd:  &FakeCommand{flagSet: &FakeFlagSet{values: map[string]any{"containerd-root-dir": "/path/to/containerd"}}},
			expected: &CfgVars{ContainerdRootDir: "/path/to/containerd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCfgVars(tt.fakeCmd, tt.dirs...)
			require.NoError(t, err)
			expected, err := filepath.Abs(tt.expected.ContainerdRootDir)
			require.NoError(t, err)
			assert.Equal(t, expected, c.ContainerdRootDir)
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
	assert.True(t, reflect.DeepEqual(v1beta1.DefaultClusterConfig(), nodeConfig))
}

func TestNodeConfig_Stdin(t *testing.T) {
	oldDefaultPath := defaultConfigPath
	defer func() { defaultConfigPath = oldDefaultPath }()
	defaultConfigPath = "/tmp/k0s.yaml.nonexistent"

	fakeCmd := &FakeCommand{
		stdin:   bytes.NewReader([]byte(`spec: {network: {provider: calico}}`)),
		flagSet: &FakeFlagSet{values: map[string]any{"config": "-"}},
	}

	underTest, err := NewCfgVars(fakeCmd)
	require.NoError(t, err)

	nodeConfig, err := underTest.NodeConfig()
	assert.NoError(t, err)
	assert.Equal(t, "calico", nodeConfig.Spec.Network.Provider)

	nodeConfig, err = underTest.NodeConfig()
	assert.ErrorContains(t, err, "stdin already grabbed")
	assert.Nil(t, nodeConfig)
}
