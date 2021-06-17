package cleanup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

// NeedsToRun checks if custom CRI is used, otherwise checks if containerd is present on the host
func (c *containers) NeedsToRun() bool {
	if c.isCustomCriUsed() {
		return true
	}
	if _, err := os.Stat(c.Config.containerd.binPath); err != nil {
		logrus.Debugf("could not find containerd binary at %v errored with: %v", c.Config.containerd.binPath, err)
		return false
	}
	return true
}

// Run removes all the pods and mounts and stops containers afterwards
// Run starts containerd if custom CRI is not configured
func (c *containers) Run() error {
	if !c.isCustomCriUsed() {
		if err := c.startContainerd(); err != nil {
			logrus.Debugf("error starting containerd: %v", err)
			return err
		}
	}

	time.Sleep(5 * time.Second)

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
		"--config=/etc/k0s/containerd.toml",
	}
	cmd := exec.Command(c.Config.containerd.binPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start containerd: %v", err)
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
	pods, err := c.Config.containerRuntime.ListContainers()
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
