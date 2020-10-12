package worker

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
)

// ContainerD implement the component interface to manage containerd as mke component
type ContainerD struct {
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (c *ContainerD) Init() error {
	for _, bin := range []string{"containerd", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2", "runc"} {
		// unfortunately, this cannot be parallelized – it will result in a fork/exec error
		err := assets.Stage(constant.BinDir, bin, constant.BinDirMode, constant.Group)
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
			"--config=/etc/mke/containerd.toml",
		},
	}
	// TODO We need to dump the config file suited for mke use

	c.supervisor.Supervise()

	return nil
}

// Stop stops containerD
func (c *ContainerD) Stop() error {
	return c.supervisor.Stop()
}
