package worker

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

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
	//var err error
	var allErrors []string
	err := assets.Stage(constant.BinDir, "containerd", constant.BinDirMode, constant.Group)
	allErrors = append(allErrors, err.Error())

	err = assets.Stage(constant.BinDir, "containerd-shim", constant.BinDirMode, constant.Group)
	allErrors = append(allErrors, err.Error())

	err = assets.Stage(constant.BinDir, "containerd-shim-runc-v1", constant.BinDirMode, constant.Group)
	allErrors = append(allErrors, err.Error())

	err = assets.Stage(constant.BinDir, "containerd-shim-runc-v2", constant.BinDirMode, constant.Group)
	allErrors = append(allErrors, err.Error())

	err = assets.Stage(constant.BinDir, "runc", constant.BinDirMode, constant.Group)
	allErrors = append(allErrors, err.Error())

	return fmt.Errorf(strings.Join(allErrors, "\n"))
}

// Run runs containerD
func (c *ContainerD) Run() error {
	logrus.Info("Starting containerD")
	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: assets.BinPath("containerd"),
		Args: []string{
			fmt.Sprintf("--root=%s", path.Join(constant.DataDir, "containerd")),
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
