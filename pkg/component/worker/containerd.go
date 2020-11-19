/*
Copyright 2020 Mirantis, Inc.

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
package worker

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// ContainerD implement the component interface to manage containerd as k0s component
type ContainerD struct {
	supervisor supervisor.Supervisor
	LogLevel   string
}

// Init extracts the needed binaries
func (c *ContainerD) Init() error {
	for _, bin := range []string{"containerd", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2", "runc"} {
		// unfortunately, this cannot be parallelized – it will result in a fork/exec error
		err := assets.Stage(constant.BinDir, bin, constant.BinDirMode)
		if err != nil {
			return err
		}
	}

	return nil
}

// Run runs containerD
func (c *ContainerD) Run() error {
	logrus.Info("Starting containerD")
	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: assets.BinPath("containerd"),
		Args: []string{
			fmt.Sprintf("--root=%s", filepath.Join(constant.DataDir, "containerd")),
			fmt.Sprintf("--state=%s", filepath.Join(constant.RunDir, "containerd")),
			fmt.Sprintf("--address=%s", filepath.Join(constant.RunDir, "containerd.sock")),
			fmt.Sprintf("--log-level=%s", c.LogLevel),
			"--config=/etc/k0s/containerd.toml",
		},
	}
	// TODO We need to dump the config file suited for k0s use

	c.supervisor.Supervise()

	return nil
}

// Stop stops containerD
func (c *ContainerD) Stop() error {
	return c.supervisor.Stop()
}

// Health-check interface
func (c *ContainerD) Healthy() error { return nil }
