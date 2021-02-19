package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

func NewCleanUpConfig(dataDir string) *CleanUpConfig {
	runDir := "/run/k0s" // https://github.com/k0sproject/k0s/pull/591/commits/c3f932de85a0b209908ad39b817750efc4987395

	return &CleanUpConfig{
		dataDir:              dataDir,
		runDir:               runDir,
		containerdSockerPath: fmt.Sprintf("%s/containerd.sock", runDir),
		criSocketPath:        fmt.Sprintf("unix:///%s/containerd.sock", runDir),
		crictlBinPath:        fmt.Sprintf("%s/%s", dataDir, "bin/crictl"),
		containerdBinPath:    fmt.Sprintf("%s/%s", dataDir, "bin/containerd"),
	}
}

func (c *CleanUpConfig) cleanupMount() error {
	var msg []string

	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}
	// search and unmount kubelet volume mounts
	for _, v := range procMounts {
		if strings.Contains(v.Path, "kubelet/pods") {
			if err = mounter.Unmount(v.Path); err != nil {
				msg = append(msg, err.Error())
			}
			if err := os.RemoveAll(v.Path); err != nil {
				msg = append(msg, err.Error())
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, ", "))
	}
	return nil
}

func (c *CleanUpConfig) cleanupNetworkNamespace() error {
	var msg []string

	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}
	// search and unmount namespace mounts
	for _, v := range procMounts {
		if strings.Contains(v.Path, "run/netns") {
			if err = mounter.Unmount(v.Path); err != nil {
				msg = append(msg, err.Error())
			}
			if err := os.RemoveAll(v.Path); err != nil {
				msg = append(msg, err.Error())
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, ", "))
	}
	return nil
}

func (c *CleanUpConfig) stopAllContainers() error {
	var msg []string

	containers, err := c.listContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		logrus.Debugf("stopping container: %v", container)
		out, err := exec.Command(c.crictlBinPath, "-r", c.criSocketPath, "stopp", container).CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "443: connect: connection refused") {
				// on a single node instance, we will see "connection refused" error. this is to be expected
				// since we're deleting the API pod itself. so we're ignoring this error
				logrus.Debugf("ignoring container stop err: %v", string(out))
			} else {
				fmtError := fmt.Errorf("failed to stop running pod %v: output: %v, err: %v", container, string(out), err)
				msg = append(msg, fmtError.Error())
			}
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, ", "))
	}
	return nil
}

func (c *CleanUpConfig) removeAllContainers() error {
	var msg []string

	containers, err := c.listContainers()
	if err != nil {
		return err
	}

	for _, container := range containers {
		out, err := exec.Command(c.crictlBinPath, "-r", c.criSocketPath, "rmp", container).CombinedOutput()
		if err != nil {
			fmtError := fmt.Errorf("failed to stop running pod %v: output: %v, err: %v", container, string(out), err)
			msg = append(msg, fmtError.Error())
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, ", "))
	}
	return nil
}

func (c *CleanUpConfig) startContainerd() error {
	args := []string{
		fmt.Sprintf("--root=%s", filepath.Join(c.dataDir, "containerd")),
		fmt.Sprintf("--state=%s", filepath.Join(c.runDir, "containerd")),
		fmt.Sprintf("--address=%s", c.containerdSockerPath),
		"--config=/etc/k0s/containerd.toml",
	}
	cmd := exec.Command(c.containerdBinPath, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start containerd: %v", err)
	}
	go func() {
		for {
			select {
			case <-c.quit:
				logrus.Debug("stopping clean-up instance of containerd...")
				if err := cmd.Process.Kill(); err != nil {
					logrus.Errorf("failed to kill containerd: %v", err)
				}
			default:
				continue
			}
		}
	}()

	return nil
}

func (c *CleanUpConfig) stopContainerd() {
	logrus.Debug("attempting to stop containerd")
	c.quit <- true

}

func (c *CleanUpConfig) listContainers() ([]string, error) {
	out, err := exec.Command(c.crictlBinPath, "-r", c.criSocketPath, "pods", "-q").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("output: %s, error: %v", string(out), err)
	}
	pods := []string{}
	pods = append(pods, strings.Fields(string(out))...)

	logrus.Debugf("got pod list: %+v", pods)
	return pods, nil
}

func (c *CleanUpConfig) RemoveAllDirectories() error {
	var msg []string

	logrus.Infof("deleting k0s generated data-dir (%v) and run-dir (%v)", c.dataDir, c.runDir)
	if err := os.RemoveAll(c.dataDir); err != nil {
		fmtError := fmt.Errorf("failed to delete %v. err: %v", c.dataDir, err)
		msg = append(msg, fmtError.Error())
	}
	if err := os.RemoveAll(c.runDir); err != nil {
		fmtError := fmt.Errorf("failed to delete %v. err: %v", c.runDir, err)
		msg = append(msg, fmtError.Error())
	}
	if len(msg) > 0 {
		return fmt.Errorf("%v", strings.Join(msg, ", "))
	}
	return nil
}
