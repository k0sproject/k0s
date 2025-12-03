// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

func NewConfig(debug bool, k0sVars *config.CfgVars, systemUsers *k0sv1beta1.SystemUser, criSocketFlag string) (*Config, error) {
	steps, err := buildSteps(debug, k0sVars, systemUsers, criSocketFlag)
	if err != nil {
		return nil, err
	}
	return &Config{cleanupSteps: steps}, nil
}

func (c *Config) Cleanup() error {
	var errs []error

	for _, step := range c.cleanupSteps {
		logrus.Info("* ", step.Name())
		if err := step.Run(); err != nil {
			logrus.Debug(err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during clean-up: %w", errors.Join(errs...))
	}
	return nil
}

func newContainersStep(debug bool, k0sVars *config.CfgVars, criSocketFlag string) (*containers, error) {
	runtimeEndpoint, err := worker.GetContainerRuntimeEndpoint(criSocketFlag, k0sVars.RunDir)
	if err != nil {
		return nil, err
	}

	containers := containers{
		containerRuntime: runtime.NewContainerRuntime(runtimeEndpoint),
	}

	if criSocketFlag == "" {
		logLevel := "error"
		if debug {
			logLevel = "debug"
		}
		containers.managedContainerd = containerd.NewComponent(logLevel, k0sVars, &workerconfig.Profile{
			PauseImage: &k0sv1beta1.ImageSpec{
				Image:   constant.KubePauseContainerImage,
				Version: constant.KubePauseContainerImageVersion,
			},
		})
	}

	return &containers, nil
}

// Step interface is used to implement cleanup steps
type Step interface {
	// Run impelements specific cleanup operations
	Run() error
	// Name returns name of the step for conveninece
	Name() string
}
