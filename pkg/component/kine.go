package component

import (
	"path"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

// Kine implement the component interface to run kine
type Kine struct {
	supervisor supervisor.Supervisor
}

// Run runs kine
func (k *Kine) Run() error {
	logrus.Info("Starting kine")
	k.supervisor = supervisor.Supervisor{
		Name:    "kine",
		BinPath: path.Join(constant.DataDir, "bin", "kine"),
	}
	// TODO We need to dump the config file suited for mke use

	k.supervisor.Supervise()

	return nil
}

// Stop stops kine
func (k *Kine) Stop() error {
	return k.supervisor.Stop()
}
