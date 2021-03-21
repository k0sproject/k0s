/*
Copyright 2021 k0s Authors

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

package install

import (
	"fmt"
	"github.com/k0sproject/k0s/pkg/crictl"
	"os/exec"
	"strings"
	"time"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/sirupsen/logrus"
)

type CleanUpConfig struct {
	containerdBinPath    string
	containerdCmd        *exec.Cmd
	containerdSockerPath string
	criSocketPath        string
	crictlBinPath        string
	criCtl               *crictl.CriCtl
	dataDir              string
	runDir               string
}

func (c *CleanUpConfig) WorkerCleanup() error {
	var msg []string

	if err := c.workerPreFlightChecks(); err != nil {
		logrus.Fatalf("failed clean up pre-flight-checks: %v", err)
	}
	logrus.Info("starting containerd for cleanup operations...")

	if err := c.startContainerd(); err != nil {
		return err
	}
	logrus.Info("containerd succesfully started")

	logrus.Info("attempting to clean up kubelet volumes...")
	if err := c.cleanupMount(); err != nil {
		logrus.Errorf("error removing kubelet mounts: %v", err)
		msg = append(msg, err.Error())
	}
	logrus.Info("successfully removed kubelet mounts!")
	logrus.Info("attempting to clean up network namespaces...")
	if err := c.cleanupNetworkNamespace(); err != nil {
		logrus.Errorf("error removing network namespaces: %v", err)
		msg = append(msg, err.Error())
	}
	logrus.Info("successfully removed network namespaces!")

	logrus.Info("attempting to stop containers...")
	time.Sleep(5 * time.Second)
	if err := c.stopAllContainers(); err != nil {
		logrus.Errorf("error stopping containers: %v", err)
		msg = append(msg, err.Error())
	}

	if err := c.removeAllContainers(); err != nil {
		logrus.Errorf("error removing containers: %v", err)
		msg = append(msg, err.Error())
	}

	containers, err := c.criCtl.ListPods()
	if err == nil && len(containers) == 0 {
		logrus.Info("successfully removed k0s containers!")
	}

	// stop containerd
	c.stopContainerd()

	if len(msg) > 0 {
		return fmt.Errorf("errors received during clean-up: %v", strings.Join(msg, ", "))
	}
	return nil
}

// This function attempts to find out the host role, by staged binaries
func GetRoleByStagedKubelet(binPath string) string {
	apiBinary := fmt.Sprintf("%s/%s", binPath, "kube-apiserver")
	kubeletBinary := fmt.Sprintf("%s/%s", binPath, "kubelet")

	if util.FileExists(apiBinary) && util.FileExists(kubeletBinary) {
		return "controller+worker"
	} else if util.FileExists(apiBinary) {
		return "controller"
	} else {
		return "worker"
	}
}

func (c *CleanUpConfig) workerPreFlightChecks() error {
	if !util.IsDirectory(c.dataDir) {
		return fmt.Errorf("failed to find %v. was this node provisioned?", c.dataDir)
	}
	if !util.FileExists(c.crictlBinPath) {
		return fmt.Errorf("failed to find %v. was this node provisioned correctly?", c.crictlBinPath)
	}
	return nil
}
