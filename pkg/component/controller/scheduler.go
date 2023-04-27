/*
Copyright 2020 k0s authors

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
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Scheduler implement the component interface to run kube scheduler
type Scheduler struct {
	gid            int
	K0sVars        *config.CfgVars
	LogLevel       string
	SingleNode     bool
	supervisor     *supervisor.Supervisor
	uid            int
	previousConfig stringmap.StringMap
}

var _ manager.Component = (*Scheduler)(nil)
var _ manager.Reconciler = (*Scheduler)(nil)

const kubeSchedulerComponentName = "kube-scheduler"

// Init extracts the needed binaries
func (a *Scheduler) Init(_ context.Context) error {
	var err error
	a.uid, err = users.GetUID(constant.SchedulerUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running kube-scheduler as root: %w", err))
	}
	return assets.Stage(a.K0sVars.BinDir, kubeSchedulerComponentName, constant.BinDirMode)
}

// Run runs kube scheduler
func (a *Scheduler) Start(_ context.Context) error {
	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	if a.supervisor != nil {
		return a.supervisor.Stop()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (a *Scheduler) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: Scheduler")

	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(a.K0sVars.CertRootDir, "scheduler.conf")
	args := stringmap.StringMap{
		"authentication-kubeconfig": schedulerAuthConf,
		"authorization-kubeconfig":  schedulerAuthConf,
		"kubeconfig":                schedulerAuthConf,
		"bind-address":              "127.0.0.1",
		"leader-elect":              "true",
		"profiling":                 "false",
		"v":                         a.LogLevel,
	}
	for name, value := range clusterConfig.Spec.Scheduler.ExtraArgs {
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding kube-scheduler flag with user provided value: %s", name)
		}
		args[name] = value
	}
	if a.SingleNode {
		args["leader-elect"] = "false"
	}
	args = clusterConfig.Spec.FeatureGates.BuildArgs(args, kubeSchedulerComponentName)

	if args.Equals(a.previousConfig) && a.supervisor != nil {
		// no changes and supervisor already running, do nothing
		logrus.WithField("component", kubeSchedulerComponentName).Info("reconcile has nothing to do")
		return nil
	}
	// Stop in case there's process running already and we need to change the config
	if a.supervisor != nil {
		logrus.WithField("component", kubeSchedulerComponentName).Info("reconcile has nothing to do")
		err := a.supervisor.Stop()
		a.supervisor = nil
		if err != nil {
			return err
		}
	}

	a.supervisor = &supervisor.Supervisor{
		Name:    kubeSchedulerComponentName,
		BinPath: assets.BinPath(kubeSchedulerComponentName, a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    args.ToDashedArgs(),
		UID:     a.uid,
		GID:     a.gid,
	}
	a.previousConfig = args
	return a.supervisor.Supervise()
}
