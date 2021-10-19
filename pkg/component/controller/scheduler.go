/*
Copyright 2021 k0s authors

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
package controller

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Scheduler implement the component interface to run kube scheduler
type Scheduler struct {
	ClusterConfig *config.ClusterConfig
	gid           int
	K0sVars       constant.CfgVars
	LogLevel      string
	supervisor    supervisor.Supervisor
	uid           int
}

// Init extracts the needed binaries
func (a *Scheduler) Init() error {
	var err error
	a.uid, err = util.GetUID(constant.SchedulerUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running kube-scheduler as root: %w", err))
	}
	return assets.Stage(a.K0sVars.BinDir, "kube-scheduler", constant.BinDirMode)
}

// Run runs kube scheduler
func (a *Scheduler) Run() error {
	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(a.K0sVars.CertRootDir, "scheduler.conf")
	args := map[string]string{
		"authentication-kubeconfig": schedulerAuthConf,
		"authorization-kubeconfig":  schedulerAuthConf,
		"kubeconfig":                schedulerAuthConf,
		"bind-address":              "127.0.0.1",
		"leader-elect":              "true",
		"profiling":                 "false",
		"v":                         a.LogLevel,
	}
	for name, value := range a.ClusterConfig.Spec.Scheduler.ExtraArgs {
		if args[name] != "" {
			logrus.Warnf("overriding kube-scheduler flag with user provided value: %s", name)
		}
		args[name] = value
	}
	var schedulerArgs []string
	for name, value := range args {
		schedulerArgs = append(schedulerArgs, fmt.Sprintf("--%s=%s", name, value))
	}
	if a.ClusterConfig.Spec.API.ExternalAddress == "" {
		schedulerArgs = append(schedulerArgs, "--leader-elect=false")
	}

	a.supervisor = supervisor.Supervisor{
		Name:    "kube-scheduler",
		BinPath: assets.BinPath("kube-scheduler", a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    schedulerArgs,
		UID:     a.uid,
		GID:     a.gid,
	}
	// TODO We need to dump the config file suited for k0s use

	return a.supervisor.Supervise()
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	return a.supervisor.Stop()
}

// Health-check interface
func (a *Scheduler) Healthy() error { return nil }
