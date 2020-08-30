package server

import (
	"fmt"
	"path"
	"path/filepath"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/supervisor"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Scheduler implement the component interface to run kube scheduler
type Scheduler struct {
	ClusterConfig *config.ClusterConfig
	supervisor    supervisor.Supervisor
	uid           int
	gid           int
}

// Init extracts the needed binaries
func (a *Scheduler) Init() error {
	var err error
	a.uid, err = util.GetUid(constant.SchedulerUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-scheduler as root"))
	}
	a.gid, _ = util.GetGid(constant.Group)

	return assets.Stage(constant.DataDir, path.Join("bin", "kube-scheduler"), constant.Group)
}

// Run runs kube scheduler
func (a *Scheduler) Run() error {
	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(constant.CertRoot, "scheduler.conf")
	args := []string{
		fmt.Sprintf("--authentication-kubeconfig=%s", schedulerAuthConf),
		fmt.Sprintf("--authorization-kubeconfig=%s", schedulerAuthConf),
		fmt.Sprintf("--kubeconfig=%s", schedulerAuthConf),
		"--bind-address=127.0.0.1",
		"--leader-elect=true",
	}
	for _, arg := range a.ClusterConfig.Spec.Scheduler.ExtraArgs {
		args = append(args, arg)
	}
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-scheduler",
		BinPath: assets.StagedBinPath(constant.DataDir, "kube-scheduler"),
		Args:    args,
		Uid:     a.uid,
		Gid:     a.gid,
	}
	// TODO We need to dump the config file suited for mke use

	a.supervisor.Supervise()

	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	return a.supervisor.Stop()
}
