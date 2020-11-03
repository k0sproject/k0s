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
package server

import (
	"fmt"
	"path/filepath"

	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
	"github.com/k0sproject/k0s/pkg/util"
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
		"profiling":                 "false",
	}
	for name, value := range a.ClusterConfig.Spec.Scheduler.ExtraArgs {
		if args[name] != "" && name != "profiling" {
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
	// TODO We need to dump the config file suited for k0s use

	a.supervisor.Supervise()

	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	return a.supervisor.Stop()
}

// Health-check interface
func (a *Scheduler) Healthy() error { return nil }
