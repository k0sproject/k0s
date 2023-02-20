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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

type containers struct {
	Config *Config
}

// Name returns the name of the step
func (c *containers) Name() string {
	return "containers steps"
}

// Run removes all the pods and mounts and stops containers afterwards
// Run starts containerd if custom CRI is not configured
func (c *containers) Run() error {
	if !c.isCustomCriUsed() {
		if err := c.startContainerd(); err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
				logrus.Debugf("containerd binary not found. Skipping container cleanup")
				return nil
			}
			return fmt.Errorf("failed to start containerd: %w", err)
		}
	}

	if err := c.stopAllContainers(); err != nil {
		logrus.Debugf("error stopping containers: %v", err)
	}

	if !c.isCustomCriUsed() {
		c.stopContainerd()
	}
	return nil
}

func removeMount(path string) error {
	var msg []string

	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}
	for _, v := range procMounts {
		if strings.Contains(v.Path, path) {
			logrus.Debugf("Unmounting: %s", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				msg = append(msg, err.Error())
			}

			logrus.Debugf("Removing: %s", v.Path)
			if err := os.RemoveAll(v.Path); err != nil {
				msg = append(msg, err.Error())
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, "\n"))
	}
	return nil
}

func (c *containers) isCustomCriUsed() bool {
	return c.Config.containerd == nil
}

func (c *containers) startContainerd() error {
	logrus.Debugf("starting containerd")
	args := []string{
		fmt.Sprintf("--root=%s", filepath.Join(c.Config.dataDir, "containerd")),
		fmt.Sprintf("--state=%s", filepath.Join(c.Config.runDir, "containerd")),
		fmt.Sprintf("--address=%s", c.Config.containerd.socketPath),
	}
	if file.Exists("/etc/k0s/containerd.toml") {
		args = append(args, "--config=/etc/k0s/containerd.toml")
	}
	cmd := exec.Command(c.Config.containerd.binPath, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	c.Config.containerd.cmd = cmd
	logrus.Debugf("started containerd successfully")

	return nil
}

func (c *containers) stopContainerd() {
	logrus.Debug("attempting to stop containerd")
	logrus.Debugf("found containerd pid: %v", c.Config.containerd.cmd.Process.Pid)
	if err := c.Config.containerd.cmd.Process.Signal(os.Interrupt); err != nil {
		logrus.Errorf("failed to kill containerd: %v", err)
	}
	// if process, didn't exit, wait a few seconds and send SIGKILL
	if c.Config.containerd.cmd.ProcessState.ExitCode() != -1 {
		time.Sleep(5 * time.Second)

		if err := c.Config.containerd.cmd.Process.Kill(); err != nil {
			logrus.Errorf("failed to send SIGKILL to containerd: %v", err)
		}
	}
	logrus.Debug("successfully stopped containerd")
}

func (c *containers) stopAllContainers() error {
	var msg []error
	logrus.Debugf("trying to list all pods")

	var pods []string
	err := retry.Do(func() error {
		var err error
		pods, err = c.Config.containerRuntime.ListContainers()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logrus.Debugf("failed at listing pods %v", err)
		return err
	}
	if len(pods) > 0 {
		if err := removeMount("kubelet/pods"); err != nil {
			msg = append(msg, err)
		}
		if err := removeMount("run/netns"); err != nil {
			msg = append(msg, err)
		}
	}

	for _, pod := range pods {
		logrus.Debugf("stopping container: %v", pod)
		err := c.Config.containerRuntime.StopContainer(pod)
		if err != nil {
			if strings.Contains(err.Error(), "443: connect: connection refused") {
				// on a single node instance, we will see "connection refused" error. this is to be expected
				// since we're deleting the API pod itself. so we're ignoring this error
				logrus.Debugf("ignoring container stop err: %v", err.Error())
			} else {
				fmtError := fmt.Errorf("failed to stop running pod %v: err: %v", pod, err)
				logrus.Debug(fmtError)
				msg = append(msg, fmtError)
			}
		}
		err = c.Config.containerRuntime.RemoveContainer(pod)
		if err != nil {
			msg = append(msg, fmt.Errorf("failed to remove pod %v: err: %v", pod, err))
		}
	}

	pods, err = c.Config.containerRuntime.ListContainers()
	if err == nil && len(pods) == 0 {
		logrus.Info("successfully removed k0s containers!")
	}

	if len(msg) > 0 {
		return fmt.Errorf("errors occurred while removing pods: %v", msg)
	}
	return nil
}
