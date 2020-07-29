package server

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/sirupsen/logrus"
)

// Scheduler implement the component interface to run kube scheduler
type Scheduler struct {
	supervisor supervisor.Supervisor
}

// Init extracts the needed binaries
func (a *Scheduler) Init() error {
	return assets.Stage(constant.DataDir, path.Join("bin", "kube-scheduler"), constant.Group)
}

// Run runs kube scheduler
func (a *Scheduler) Run() error {
	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(constant.CertRoot, "scheduler.conf")
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-scheduler",
		BinPath: path.Join(constant.DataDir, "bin", "kube-scheduler"),
		Args: []string{
			fmt.Sprintf("--authentication-kubeconfig=%s", schedulerAuthConf),
			fmt.Sprintf("--authorization-kubeconfig=%s", schedulerAuthConf),
			fmt.Sprintf("--kubeconfig=%s", schedulerAuthConf),
			"--bind-address=127.0.0.1",
			"--leader-elect=true",
		},
	}
	// TODO We need to dump the config file suited for mke use

	a.supervisor.Supervise()

	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	return a.supervisor.Stop()
}
