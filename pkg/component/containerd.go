package component

import (
	"path"

	"github.com/sirupsen/logrus"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
)

type ContainerD struct {
	supervisor supervisor.Supervisor
}

func (c ContainerD) Run() error {
	logrus.Info("Starting containerD")
	c.supervisor = supervisor.Supervisor{
		Name:    "containerd",
		BinPath: path.Join(constant.DataDir, "bin", "containerd"),
	}
	// TODO We need to dump the config file suited for mke use

	c.supervisor.Supervise()

	return nil
}

func (c ContainerD) Stop() error {
	return c.supervisor.Stop()
}
