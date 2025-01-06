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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/shirou/gopsutil/v4/process"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

const (
	RuntimeConfigKind = "RuntimeConfig"
)

var (
	ErrK0sNotRunning        = errors.New("k0s is not running")
	ErrK0sAlreadyRunning    = errors.New("an instance of k0s is already running")
	ErrInvalidRuntimeConfig = errors.New("invalid runtime config")
)

// Runtime config is a static copy of the start up config and CfgVars that is used by
// commands that do not have a --config parameter of their own, such as `k0s token create`.
// It also stores the k0svars, so the original parameters for the controller such as
// `--data-dir` will be reused by the commands without the user having to specify them again.
type RuntimeConfig struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`

	Spec *RuntimeConfigSpec `json:"spec"`
}

type RuntimeConfigSpec struct {
	NodeConfig *v1beta1.ClusterConfig `json:"nodeConfig"`
	K0sVars    *CfgVars               `json:"k0sVars"`
	Pid        int                    `json:"pid"`
}

func LoadRuntimeConfig(k0sVars *CfgVars) (*RuntimeConfigSpec, error) {
	content, err := os.ReadFile(k0sVars.RuntimeConfigPath)
	if err != nil {
		return nil, err
	}

	// "migrate" old runtime config to allow running commands on a new binary while an old version is still running.
	// the legacy runtime config gets deleted when the server running on the old binary is stopped.
	if isLegacy(content) {
		return migrateLegacyRuntimeConfig(k0sVars, content)
	}

	config := &RuntimeConfig{}
	if err := yaml.Unmarshal(content, config); err != nil {
		return nil, err
	}

	if config.APIVersion != v1beta1.ClusterConfigAPIVersion {
		return nil, fmt.Errorf("%w: invalid api version: %s", ErrInvalidRuntimeConfig, config.APIVersion)
	}

	if config.Kind != RuntimeConfigKind {
		return nil, fmt.Errorf("%w: invalid kind: %s", ErrInvalidRuntimeConfig, config.Kind)
	}

	spec := config.Spec
	if spec == nil {
		return nil, fmt.Errorf("%w: spec is nil", ErrInvalidRuntimeConfig)
	}

	// If a pid is defined but there's no process found, the instance of k0s is
	// expected to have died, in which case the existing config is removed and
	// an error is returned, which allows the controller startup to proceed to
	// initialize a new runtime config.
	if spec.Pid != 0 {
		isRunning, err := checkInstanceRunning(spec.Pid)
		if !isRunning || err != nil {
			defer func() { _ = spec.Cleanup() }()
			if !isRunning {
				return nil, errors.Join(ErrK0sNotRunning, err)
			} else if err != nil {
				return nil, fmt.Errorf("%w: failed to check if instance is running", err)
			}
		}
	}

	return spec, nil
}

func migrateLegacyRuntimeConfig(k0sVars *CfgVars, content []byte) (*RuntimeConfigSpec, error) {
	cfg := &v1beta1.ClusterConfig{}

	if err := yaml.Unmarshal(content, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legacy runtime config: %w", err)
	}

	// generate a new runtime config
	return &RuntimeConfigSpec{K0sVars: k0sVars, NodeConfig: cfg, Pid: os.Getpid()}, nil
}

func isLegacy(data []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "kind:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
			return value != RuntimeConfigKind
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error scanning runtime config:", err)
	}

	return false
}

func NewRuntimeConfig(k0sVars *CfgVars) (*RuntimeConfigSpec, error) {
	if _, err := LoadRuntimeConfig(k0sVars); err == nil {
		return nil, ErrK0sAlreadyRunning
	}

	nodeConfig, err := k0sVars.NodeConfig()
	if err != nil {
		return nil, fmt.Errorf("load node config: %w", err)
	}

	vars := k0sVars.DeepCopy()

	// don't persist the startup config path in the runtime config
	vars.StartupConfigPath = ""

	cfg := &RuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Now(),
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta1.ClusterConfigAPIVersion,
			Kind:       RuntimeConfigKind,
		},
		Spec: &RuntimeConfigSpec{
			NodeConfig: nodeConfig,
			K0sVars:    k0sVars,
			Pid:        os.Getpid(),
		},
	}

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	if err := dir.Init(filepath.Dir(k0sVars.RuntimeConfigPath), constant.RunDirMode); err != nil {
		logrus.Warnf("failed to initialize runtime config dir: %v", err)
	}

	if err := os.WriteFile(k0sVars.RuntimeConfigPath, content, 0600); err != nil {
		return nil, fmt.Errorf("failed to write runtime config: %w", err)
	}

	return cfg.Spec, nil
}

func (r *RuntimeConfigSpec) Cleanup() error {
	if r == nil || r.K0sVars == nil {
		return nil
	}

	if err := os.Remove(r.K0sVars.RuntimeConfigPath); err != nil {
		return fmt.Errorf("failed to clean up runtime config file: %w", err)
	}
	return nil
}

// returns:
// - true, nil if the pid is the same k0s executable and running
// - false, nil if the pid is not running or another executable
// - false, error on any kind of internal error checking the status
//
// process does not exist -> error
// process exists and is same executable -> error
// process exists and is not same executable -> nil
func checkInstanceRunning(pid int) (bool, error) {
	// create the process object
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return false, fmt.Errorf("failed to create process object: %w", err)
	}

	// first check if the pid is running
	isRunning, err := proc.IsRunning()
	if err != nil {
		return false, fmt.Errorf("failed to check if process is running: %w", err)
	}
	if !isRunning {
		return false, nil
	}

	// Get the executable path of the target pid
	// - no need to resolve symlinks here, it is already a resolved path
	exeTarget, err := proc.Exe()
	if err != nil {
		return false, fmt.Errorf("failed to get process executable path: %w", err)
	}

	// Get the executable path of the current process
	currentExe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to get current executable: %w", err)
	}

	// Resolve symlinks
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return false, fmt.Errorf("failed to resolve current executable symlinks: %w", err)
	}

	// check that the executable is the same in order to issue an error, if it would be a different
	// than it can't be a running instance of k0s, which the check is all about
	if exeTarget != currentExe {
		return false, nil
	}

	// return that k0s is running witht he same executable image
	return true, nil
}
