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
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/container/runtime"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
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

func removeMount(path string) error {
	var errs []error

	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}
	for _, v := range procMounts {
		if strings.Contains(v.Path, path) {
			logrus.Debugf("Unmounting: %s", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				errs = append(errs, err)
			}

			logrus.Debugf("Removing: %s", v.Path)
			if err := os.RemoveAll(v.Path); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
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
		if err := removeMount("run/netns"); err != nil {
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
