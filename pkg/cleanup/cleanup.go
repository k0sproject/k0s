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
	"errors"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/container/runtime"

	"github.com/sirupsen/logrus"
)

type Config struct {
	cleanupSteps []Step
}

func NewConfig(debug bool, k0sVars *config.CfgVars, criSocketFlag string) (*Config, error) {
	cfg, err := k0sVars.NodeConfig()
	if err != nil {
		logrus.Errorf("failed to get cluster setup: %v", err)
	}

	runtimeEndpoint, err := worker.GetContainerRuntimeEndpoint(criSocketFlag, k0sVars.RunDir)
	if err != nil {
		return nil, err
	}

	containerRuntime := runtime.NewContainerRuntime(runtimeEndpoint)
	var managedContainerd *containerd.Component
	if criSocketFlag == "" {
		logLevel := "error"
		if debug {
			logLevel = "debug"
		}
		managedContainerd = containerd.NewComponent(logLevel, k0sVars, &workerconfig.Profile{
			PauseImage: &k0sv1beta1.ImageSpec{
				Image:   constant.KubePauseContainerImage,
				Version: constant.KubePauseContainerImageVersion,
			},
		})
	}

	cleanupSteps := []Step{
		&containers{
			managedContainerd: managedContainerd,
			containerRuntime:  containerRuntime,
		},
		&users{
			systemUsers: cfg.Spec.Install.SystemUsers,
		},
		&services{},
		&directories{
			dataDir: k0sVars.DataDir,
			runDir:  k0sVars.RunDir,
		},
		&cni{},
	}

	if bridge := newBridgeStep(); bridge != nil {
		cleanupSteps = append(cleanupSteps, bridge)
	}

	return &Config{cleanupSteps}, nil
}

func (c *Config) Cleanup() error {
	var errs []error

	for _, step := range c.cleanupSteps {
		logrus.Info("* ", step.Name())
		err := step.Run()
		if err != nil {
			logrus.Debug(err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during clean-up: %w", errors.Join(errs...))
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
