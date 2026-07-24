//go:build linux || windows

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/container/runtime"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
)

// containerStopTimeout bounds each CRI stop or remove call so a stuck CNI
// teardown cannot hang reset. Kept under containerd 60s CNI timeout.
const containerStopTimeout = 30 * time.Second

type containers struct {
	managedContainerd *containerd.Component
	containerRuntime  runtime.ContainerRuntime

	stopTimeout   time.Duration // overridable in tests
	cleanupMounts func() error  // overridable in tests
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
		if err := c.cleanupMounts(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, pod := range pods {
		logrus.Debugf("stopping container: %v", pod)
		if err := c.stopContainer(ctx, pod); err != nil {
			if isExpectedStopError(err) {
				// expected during reset, API gone or CNI teardown timed out
				logrus.Debugf("ignoring pod %s stop error: %v", pod, err)
			} else {
				errs = append(errs, fmt.Errorf("failed to stop running pod %s: %w", pod, err))
			}
		}
		if err := c.removeContainer(ctx, pod); err != nil {
			if isExpectedStopError(err) {
				logrus.Debugf("ignoring pod %s remove error: %v", pod, err)
			} else {
				errs = append(errs, fmt.Errorf("failed to remove pod %s: %w", pod, err))
			}
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

// stopContainer stops one pod sandbox under a deadline.
func (c *containers) stopContainer(ctx context.Context, pod string) error {
	ctx, cancel := context.WithTimeout(ctx, c.stopTimeout)
	defer cancel()
	return c.containerRuntime.StopContainer(ctx, pod)
}

// removeContainer removes one pod sandbox under a deadline.
func (c *containers) removeContainer(ctx context.Context, pod string) error {
	ctx, cancel := context.WithTimeout(ctx, c.stopTimeout)
	defer cancel()
	return c.containerRuntime.RemoveContainer(ctx, pod)
}

// isExpectedStopError reports whether a stop or remove error is a normal
// consequence of tearing down the control plane and can be ignored.
func isExpectedStopError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "443: connect: connection refused"):
		return true
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "the server was unable to return a response in the time allotted"):
		return true
	default:
		return false
	}
}

func newContainersStep(debug bool, k0sVars *config.CfgVars, criSocketFlag string) (*containers, error) {
	runtimeEndpoint, err := worker.GetContainerRuntimeEndpoint(criSocketFlag, k0sVars.RunDir)
	if err != nil {
		return nil, err
	}

	containers := containers{
		containerRuntime: runtime.NewContainerRuntime(runtimeEndpoint),
		stopTimeout:      containerStopTimeout,
		cleanupMounts:    cleanupContainerMounts,
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
