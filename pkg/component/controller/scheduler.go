// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/scheduler/plugins/dynamicport"
)

// Scheduler implement the component interface to run kube scheduler
type Scheduler struct {
	K0sVars               *config.CfgVars
	LogLevel              string
	DisableLeaderElection bool

	previousConfig stringmap.StringMap
	cancelFunc     context.CancelFunc
}

var _ manager.Component = (*Scheduler)(nil)
var _ manager.Reconciler = (*Scheduler)(nil)

const kubeSchedulerComponentName = "kube-scheduler"

// Init initializes the component
func (a *Scheduler) Init(_ context.Context) error {
	return nil
}

// Start runs kube scheduler
func (a *Scheduler) Start(_ context.Context) error {
	return nil
}

// Stop stops Scheduler
func (a *Scheduler) Stop() error {
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (a *Scheduler) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logrus.Debug("reconcile method called for: Scheduler")

	logrus.Info("Starting kube-scheduler")
	schedulerAuthConf := filepath.Join(a.K0sVars.CertRootDir, "scheduler.conf")
	
	// Generate KubeSchedulerConfiguration to enable DynamicPort plugin
	schedulerConfigPath := filepath.Join(a.K0sVars.RunDir, "scheduler-config.yaml")
	configContent := fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: "%s"
leaderElection:
  leaderElect: %s
enableProfiling: false
profiles:
  - schedulerName: default-scheduler
    plugins:
      reserve:
        enabled:
          - name: DynamicPort
`, schedulerAuthConf, "true")

	if a.DisableLeaderElection {
		configContent = fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: "%s"
leaderElection:
  leaderElect: %s
enableProfiling: false
profiles:
  - schedulerName: default-scheduler
    plugins:
      reserve:
        enabled:
          - name: DynamicPort
`, schedulerAuthConf, "false")
	}

	if err := os.WriteFile(schedulerConfigPath, []byte(configContent), 0600); err != nil {
		return err
	}

	args := stringmap.StringMap{
		"authentication-kubeconfig": schedulerAuthConf,
		"authorization-kubeconfig":  schedulerAuthConf,
		"config":                    schedulerConfigPath,
		"bind-address":              "127.0.0.1",
		"v":                         a.LogLevel,
	}
	// Remove flags that are now in config
	// kubeconfig, leader-elect, profiling are handled in config
	
	for name, value := range clusterConfig.Spec.Scheduler.ExtraArgs {
		if _, ok := args[name]; ok {
			logrus.Warnf("overriding kube-scheduler flag with user provided value: %s", name)
		}
		args[name] = value
	}

	args = clusterConfig.Spec.FeatureGates.BuildArgs(args, kubeSchedulerComponentName)

	if args.Equals(a.previousConfig) && a.cancelFunc != nil {
		// no changes and scheduler already running, do nothing
		logrus.WithField("component", kubeSchedulerComponentName).Info("reconcile has nothing to do")
		return nil
	}
	// Stop in case there's process running already and we need to change the config
	if a.cancelFunc != nil {
		logrus.WithField("component", kubeSchedulerComponentName).Info("Restarting scheduler with new config")
		a.cancelFunc()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel

	cmdArgs := append(args.ToDashedArgs(), clusterConfig.Spec.Scheduler.RawArgs...)

	go func() {
		// Run embedded scheduler with DynamicPort plugin
		command := app.NewSchedulerCommand(
			app.WithPlugin(dynamicport.Name, dynamicport.New),
		)
		command.SetArgs(cmdArgs)
		// Suppress stdout/stderr to avoid log spam if needed, or pipe to logrus?
		// For now let it print to stderr.
		if err := command.ExecuteContext(ctx); err != nil {
			// Context cancellation creates an error too, ignore it
			if ctx.Err() == nil {
				logrus.Errorf("Scheduler failed: %v", err)
			}
		}
	}()

	a.previousConfig = args
	return nil
}

