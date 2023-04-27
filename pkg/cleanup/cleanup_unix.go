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

package cleanup

import (
	"fmt"
	"os/exec"

	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/k0sproject/k0s/pkg/container/runtime"
	"github.com/sirupsen/logrus"
)

type Config struct {
	cfgFile          string
	containerd       *containerdConfig
	containerRuntime runtime.ContainerRuntime
	dataDir          string
	k0sVars          *config.CfgVars
	runDir           string
}

type containerdConfig struct {
	binPath    string
	cmd        *exec.Cmd
	socketPath string
}

func NewConfig(k0sVars *config.CfgVars, cfgFile string, criSocketPath string) (*Config, error) {
	runDir := "/run/k0s" // https://github.com/k0sproject/k0s/pull/591/commits/c3f932de85a0b209908ad39b817750efc4987395

	var err error
	var containerdCfg *containerdConfig
	var runtimeType string

	if criSocketPath == "" {
		criSocketPath = fmt.Sprintf("unix://%s/containerd.sock", runDir)
		containerdCfg = &containerdConfig{
			binPath:    fmt.Sprintf("%s/%s", k0sVars.DataDir, "bin/containerd"),
			socketPath: fmt.Sprintf("%s/containerd.sock", runDir),
		}
		runtimeType = "cri"
	} else {
		runtimeType, criSocketPath, err = worker.SplitRuntimeConfig(criSocketPath)
		if err != nil {
			return nil, err
		}
	}

	return &Config{
		cfgFile:          cfgFile,
		containerd:       containerdCfg,
		containerRuntime: runtime.NewContainerRuntime(runtimeType, criSocketPath),
		dataDir:          k0sVars.DataDir,
		runDir:           runDir,
		k0sVars:          k0sVars,
	}, nil
}

func (c *Config) Cleanup() error {
	var msg []error
	cleanupSteps := []Step{
		&containers{Config: c},
		&users{Config: c},
		&services{Config: c},
		&directories{Config: c},
		&cni{},
		&bridge{},
	}

	for _, step := range cleanupSteps {
		logrus.Info("* ", step.Name())
		err := step.Run()
		if err != nil {
			logrus.Debug(err)
			msg = append(msg, err)
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("errors received during clean-up: %v", msg)
	}
	return nil
}

// Step interface is used to implement cleanup steps
type Step interface {
	// Run impelements specific cleanup operations
	Run() error
	// Name returns name of the step for conveninece
	Name() string
}
