//go:build linux || windows

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/container/runtime"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

type containers struct {
	managedContainerd *containerd.Component
	containerRuntime  runtime.ContainerRuntime
}

// Name returns the name of the step
func (c *containers) Name() string {
	return "containers steps"
}

// Run removes all the pods and mounts and stops containers afterwards
// Run starts containerd if custom CRI is not configured
func (c *containers) Run() error {
	if c.managedContainerd != nil {
		ctx := context.TODO()
		if err := c.managedContainerd.Init(ctx); err != nil {
			logrus.WithError(err).Warn("Failed to initialize containerd, skipping container cleanup")
			return nil
		}
		if err := c.managedContainerd.Start(ctx); err != nil {
			logrus.WithError(err).Warn("Failed to start containerd, skipping container cleanup")
			return nil
		}
		defer func() {
			if err := c.managedContainerd.Stop(); err != nil {
				logrus.WithError(err).Warn("Failed to stop containerd")
			}
		}()
	}

	if err := c.stopAllContainers(); err != nil {
		logrus.Debugf("error stopping containers: %v", err)
	}

	return nil
}

func (c *containers) stopAllContainers() error {
	var errs []error

	var pods []string
	ctx := context.TODO()
	err := retry.Do(func() error {
		logrus.Debugf("trying to list all pods")
		var err error
		pods, err = c.containerRuntime.ListContainers(ctx)
		if err != nil {
			return err
		}
		return nil
	}, retry.Context(ctx), retry.LastErrorOnly(true))
	if err != nil {
		return fmt.Errorf("failed at listing pods %w", err)
	}
	if len(pods) > 0 {
		if err := cleanupContainerMounts(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, pod := range pods {
		logrus.Debugf("stopping container: %v", pod)
		err := c.containerRuntime.StopContainer(ctx, pod)
		if err != nil {
			if strings.Contains(err.Error(), "443: connect: connection refused") {
				// on a single node instance, we will see "connection refused" error. this is to be expected
				// since we're deleting the API pod itself. so we're ignoring this error
				logrus.Debugf("ignoring container stop err: %v", err.Error())
			} else {
				errs = append(errs, fmt.Errorf("failed to stop running pod %s: %w", pod, err))
			}
		}
		err = c.containerRuntime.RemoveContainer(ctx, pod)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to remove pod %s: %w", pod, err))
		}
	}

	pods, err = c.containerRuntime.ListContainers(ctx)
	if err == nil && len(pods) == 0 {
		logrus.Info("successfully removed k0s containers!")
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred while removing pods: %w", errors.Join(errs...))
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
			PauseImage: defaultPauseImage(),
		})
	}

	return &containers, nil
}
