package component

import (
	"path"

	"github.com/sirupsen/logrus"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
)

// ContainerD implement the component interface to manage containerd as mke component
type ContainerD struct {
	supervisor supervisor.Supervisor
}

// Run runs containerD
func (c *ContainerD) Run() error {
	logrus.Info("Starting containerD")
	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: path.Join(constant.DataDir, "bin", "containerd"),
	}
	// TODO We need to dump the config file suited for mke use

	c.supervisor.Supervise()

	return nil
}

// Stop stops containerD
func (c *ContainerD) Stop() error {
	return c.supervisor.Stop()
}
