package server

import (
	"fmt"
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
	a.uid, err = util.GetUID(constant.SchedulerUser)
	if err != nil {
		logrus.Warning(errors.Wrap(err, "Running kube-scheduler as root"))
	}
	a.gid, _ = util.GetGID(constant.Group)

	return assets.Stage(constant.BinDir, "kube-scheduler", constant.BinDirMode, constant.Group)
}

// Run runs kube scheduler
func (a *Scheduler) Run() error {
	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(constant.CertRootDir, "scheduler.conf")
	args := map[string]string{
		"authentication-kubeconfig": schedulerAuthConf,
		"authorization-kubeconfig":  schedulerAuthConf,
		"kubeconfig":                schedulerAuthConf,
		"bind-address":              "127.0.0.1",
		"leader-elect":              "true",
	}
	for name, value := range a.ClusterConfig.Spec.Scheduler.ExtraArgs {
		if args[name] != "" {
			return fmt.Errorf("cannot override kube-scheduler flag: %s", name)
		}
		args[name] = value
	}
	schedulerArgs := []string{}
	for name, value := range args {
		schedulerArgs = append(schedulerArgs, fmt.Sprintf("--%s=%s", name, value))
	}
	a.supervisor = supervisor.Supervisor{
		Name:    "kube-scheduler",
		BinPath: assets.BinPath("kube-scheduler"),
		Args:    schedulerArgs,
		UID:     a.uid,
		GID:     a.gid,
	}
	// TODO We need to dump the config file suited for mke use

	a.supervisor.Supervise()

	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	return a.supervisor.Stop()
}

// Health-check interface
func (a *Scheduler) Healthy() error { return nil }
