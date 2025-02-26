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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

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
	ErrInvalidRuntimeConfig = errors.New("invalid runtime configuration")
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
	lockFile   *os.File
}

func LoadRuntimeConfig(path string) (*RuntimeConfigSpec, error) {
	if !isLocked(path + ".lock") {
		return nil, ErrK0sNotRunning
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config, err := ParseRuntimeConfig(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse runtime configuration: %w", err)
	}

	return config.Spec, nil
}

func ParseRuntimeConfig(content []byte) (*RuntimeConfig, error) {
	var config RuntimeConfig

	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	if config.APIVersion != v1beta1.ClusterConfigAPIVersion {
		return nil, fmt.Errorf("%w: invalid api version: %q", ErrInvalidRuntimeConfig, config.APIVersion)
	}

	if config.Kind != RuntimeConfigKind {
		return nil, fmt.Errorf("%w: invalid kind: %q", ErrInvalidRuntimeConfig, config.Kind)
	}

	if config.Spec == nil {
		return nil, fmt.Errorf("%w: spec is nil", ErrInvalidRuntimeConfig)
	}

	return &config, nil
}

func NewRuntimeConfig(k0sVars *CfgVars, nodeConfig *v1beta1.ClusterConfig) (*RuntimeConfig, error) {
	// A file lock is acquired using `flock(2)` to ensure that only one
	// instance of the `k0s` process can modify the runtime configuration
	// at a time. The lock is tied to the lifetime of the `k0s` process,
	// meaning that if the process terminates unexpectedly, the lock is
	// automatically released by the operating system. This ensures that
	// subsequent processes can acquire the lock without manual cleanup.
	// https://man7.org/linux/man-pages/man2/flock.2.html
	//
	// It works similar on Windows, but with LockFileEx

	path, err := filepath.Abs(k0sVars.RuntimeConfigPath + ".lock")
	if err != nil {
		return nil, err
	}
	lockFile, err := tryLock(path)
	if err != nil {
		return nil, err
	}

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
			lockFile:   lockFile,
		},
	}

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(k0sVars.RuntimeConfigPath, content, 0600); err != nil {
		return nil, fmt.Errorf("failed to write runtime config: %w", err)
	}

	return cfg, nil
}

func (r *RuntimeConfigSpec) Cleanup() error {
	if r == nil || r.K0sVars == nil {
		return nil
	}

	if err := os.Remove(r.K0sVars.RuntimeConfigPath); err != nil {
		logrus.Warnf("failed to clean up runtime config file: %v", err)
	}

	if err := r.lockFile.Close(); err != nil {
		return fmt.Errorf("failed to close the runtime config file: %w", err)
	}

	if err := os.Remove(r.lockFile.Name()); err != nil {
		return fmt.Errorf("failed to delete %s: %w", r.lockFile.Name(), err)
	}
	return nil
}
