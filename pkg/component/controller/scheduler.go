// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
	K0sVars               *config.CfgVars
	LogLevel              string
	DisableLeaderElection bool

	supervisor     *supervisor.Supervisor
	executablePath string
	uid            int
	previousConfig stringmap.StringMap
}

var _ manager.Component = (*Scheduler)(nil)
var _ manager.Reconciler = (*Scheduler)(nil)

const kubeSchedulerComponentName = "kube-scheduler"

// Init extracts the needed binaries
func (a *Scheduler) Init(_ context.Context) error {
	var err error
	a.uid, err = users.LookupUID(constant.SchedulerUser)
	if err != nil {
		err = fmt.Errorf("failed to lookup UID for %q: %w", constant.SchedulerUser, err)
		a.uid = users.RootUID
		logrus.WithError(err).Warn("Running kube-scheduler as root")
	}
	a.executablePath, err = assets.StageExecutable(a.K0sVars.BinDir, kubeSchedulerComponentName)
	return err
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
func (a *Scheduler) Reconcile(ctx context.Context, clusterConfig *v1beta1.ClusterConfig) error {
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
	if a.DisableLeaderElection {
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
		if err := a.supervisor.Stop(); err != nil {
			logrus.WithField("component", kubeSchedulerComponentName).WithError(err).Error("Failed to stop executable")
		}
		a.supervisor = nil
	}

	a.supervisor = &supervisor.Supervisor{
		Name:    kubeSchedulerComponentName,
		BinPath: a.executablePath,
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    append(args.ToDashedArgs(), clusterConfig.Spec.Scheduler.RawArgs...),
		UID:     a.uid,
	}
	a.previousConfig = args
	return a.supervisor.Supervise(ctx)
}
