/*
Copyright 2020 k0s authors

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
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

const confTmpl = `
# This is a placeholder configuration for k0s managed containerD.
# If you wish to customize the config replace this file with your custom configuration.
# For reference see https://github.com/containerd/containerd/blob/main/docs/man/containerd-config.toml.5.md
version = 2
`
const confPath = "/etc/k0s/containerd.toml"

// ContainerD implement the component interface to manage containerd as k0s component
type ContainerD struct {
	supervisor supervisor.Supervisor
	LogLevel   string
	K0sVars    constant.CfgVars

	OCIBundlePath string
}

var _ manager.Component = (*ContainerD)(nil)

// Init extracts the needed binaries
func (c *ContainerD) Init(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	for _, bin := range []string{"containerd", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2", "runc"} {
		b := bin
		g.Go(func() error {
			return assets.Stage(c.K0sVars.BinDir, b, constant.BinDirMode)
		})
	}

	return g.Wait()
}

// Run runs containerD
func (c *ContainerD) Start(_ context.Context) error {
	logrus.Info("Starting containerD")

	if err := c.setupConfig(); err != nil {
		return err
	}

	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: assets.BinPath("containerd", c.K0sVars.BinDir),
		RunDir:  c.K0sVars.RunDir,
		DataDir: c.K0sVars.DataDir,
		Args: []string{
			fmt.Sprintf("--root=%s", filepath.Join(c.K0sVars.DataDir, "containerd")),
			fmt.Sprintf("--state=%s", filepath.Join(c.K0sVars.RunDir, "containerd")),
			fmt.Sprintf("--address=%s", filepath.Join(c.K0sVars.RunDir, "containerd.sock")),
			fmt.Sprintf("--log-level=%s", c.LogLevel),
			fmt.Sprintf("--config=%s", confPath),
		},
	}

	return c.supervisor.Supervise()
}

func (c *ContainerD) setupConfig() error {
	// If the config file exists, use it as-is
	if file.Exists(confPath) {
		return nil
	}

	if err := dir.Init(filepath.Dir(confPath), 0755); err != nil {
		return err
	}
	return file.WriteContentAtomically(confPath, []byte(confTmpl), 0644)
}

// Stop stops containerD
func (c *ContainerD) Stop() error {
	return c.supervisor.Stop()
}
