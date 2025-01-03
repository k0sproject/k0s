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
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
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

// The idea behind this test is:
// - test to create a new runtime config from scratch
// - actually test if k0s is still running by checking against:
//   - the same executable with the same pid as the runtime config
//   - a different executable with the same pid as the runtime config
//   - a pid that is not running anymore
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
	// this will set the pid property to the current process id of the test run
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
	// this will fail with ErrK0sAlreadyRunning since the pid property is this currently
	// executing test process (with the unit test)
	_, err = NewRuntimeConfig(k0sVars)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrK0sAlreadyRunning)

	// Start a process in the background with a shell that sleeps for a long time
	// (infinity is not available on default macOS zsh), this simulates
	// a different running process with the same pid as stored in the runtime config, but
	// from a different executable image
	cmd := getBackgroundCommand()
	err = cmd.Start()
	require.NoError(t, err, "Failed to start process")

	// remember the newly started PID
	pid := cmd.Process.Pid
	cleanupPid := pid

	// create the process object
	proc, err := process.NewProcess(int32(pid))
	require.NoError(t, err, "failed to create process object")

	// check that the pid is running
	isRunning, err := proc.IsRunning()
	require.NoError(t, err, "IsRunning works correctly for newly started background process")
	require.True(t, isRunning, "background process is running")

	// this is the cleanup function that cleans the process in case of a test failure
	cleanupFunc := func() {
		if cleanupPid != 0 {
			// Shut down the child process (killing it is fine here)
			err = proc.Kill()
			require.NoError(t, err, "Failed to shut down child process")

			// Wait for the command to exit (ingore the error, since we're just cleaning up)
			_ = cmd.Wait()

			// set the cleanupPid to 0 to indicate successful termination and not clean up twice
			cleanupPid = 0
		}
	}
	defer cleanupFunc()

	// modify the pid in the runtime config
	err = updateRuntimeConfigPID(k0sVars.RuntimeConfigPath, pid)
	require.NoError(t, err)

	// create a new runtime config, but this time with a pid of a process that actually exists
	// but is from a different executable image
	_, err = NewRuntimeConfig(k0sVars)
	require.NoError(t, err)

	// now kill the other process in order to test for the same pid, with the difference
	// that we now know it's from a process that is definitely not running anymore
	cleanupFunc()

	// modify the pid in the runtime config (again to the same pid, which is not running anymore)
	// this needs to be done since the previous PID value was overwritten with the new
	// config from the last call to NewRuntimeConfig)
	err = updateRuntimeConfigPID(k0sVars.RuntimeConfigPath, pid)
	require.NoError(t, err)

	// create a new runtime config, this should also succeed since the pid is not running
	_, err = NewRuntimeConfig(k0sVars)
	assert.NoError(t, err)
}

func updateRuntimeConfigPID(path string, pid int) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	config := &RuntimeConfig{}
	err = yaml.Unmarshal(content, config)
	if err != nil {
		return err
	}
	config.Spec.Pid = pid

	content, err = yaml.Marshal(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, content, 0600)
	if err != nil {
		return err
	}
	return nil
}
