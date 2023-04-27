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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Manager implement the component interface to run kube scheduler
type Manager struct {
	K0sVars               *config.CfgVars
	LogLevel              string
	SingleNode            bool
	ServiceClusterIPRange string
	ExtraArgs             string

	supervisor     *supervisor.Supervisor
	uid, gid       int
	previousConfig stringmap.StringMap
}

var cmDefaultArgs = stringmap.StringMap{
	"allocate-node-cidrs":             "true",
	"bind-address":                    "127.0.0.1",
	"cluster-name":                    "k0s",
	"controllers":                     "*,bootstrapsigner,tokencleaner",
	"enable-hostpath-provisioner":     "true",
	"leader-elect":                    "true",
	"use-service-account-credentials": "true",
}

const kubeControllerManagerComponent = "kube-controller-manager"

var _ manager.Component = (*Manager)(nil)
var _ manager.Reconciler = (*Manager)(nil)

// Init extracts the needed binaries
func (a *Manager) Init(_ context.Context) error {
	var err error
	// controller manager running as api-server user as they both need access to same sa.key
	a.uid, err = users.GetUID(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("running kube-controller-manager as root: %w", err))
	}

	// controller manager should be the only component that needs access to
	// ca.key so let it own it.
	if err := os.Chown(path.Join(a.K0sVars.CertRootDir, "ca.key"), a.uid, -1); err != nil && os.Geteuid() == 0 {
		logrus.Warning(fmt.Errorf("failed to change permissions for the ca.key: %w", err))
	}
	return assets.Stage(a.K0sVars.BinDir, kubeControllerManagerComponent, constant.BinDirMode)
}

// Run runs kube Manager
func (a *Manager) Start(_ context.Context) error { return nil }

// Reconcile detects changes in configuration and applies them to the component
func (a *Manager) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	logger := logrus.WithField("component", kubeControllerManagerComponent)
	logger.Info("Starting reconcile")
	ccmAuthConf := filepath.Join(a.K0sVars.CertRootDir, "ccm.conf")
	args := stringmap.StringMap{
		"authentication-kubeconfig":        ccmAuthConf,
		"authorization-kubeconfig":         ccmAuthConf,
		"kubeconfig":                       ccmAuthConf,
		"client-ca-file":                   path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"cluster-signing-cert-file":        path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"cluster-signing-key-file":         path.Join(a.K0sVars.CertRootDir, "ca.key"),
		"requestheader-client-ca-file":     path.Join(a.K0sVars.CertRootDir, "front-proxy-ca.crt"),
		"root-ca-file":                     path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"service-account-private-key-file": path.Join(a.K0sVars.CertRootDir, "sa.key"),
		"cluster-cidr":                     clusterConfig.Spec.Network.BuildPodCIDR(),
		"service-cluster-ip-range":         a.ServiceClusterIPRange,
		"profiling":                        "false",
		"terminated-pod-gc-threshold":      "12500",
		"v":                                a.LogLevel,
	}

	// Handle the extra args as last so they can be used to overrride some k0s "hardcodings"
	if a.ExtraArgs != "" {
		// This service uses args without hyphens, so enforce that.
		extras := flags.Split(strings.ReplaceAll(a.ExtraArgs, "--", ""))
		args.Merge(extras)
	}

	if clusterConfig.Spec.Network.DualStack.Enabled {
		args["node-cidr-mask-size-ipv6"] = "110"
		args["node-cidr-mask-size-ipv4"] = "24"
	} else {
		args["node-cidr-mask-size"] = "24"
	}
	for name, value := range clusterConfig.Spec.ControllerManager.ExtraArgs {
		if _, ok := args[name]; ok {
			logger.Warnf("overriding kube-controller-manager flag with user provided value: %s", name)
		}
		args[name] = value
	}
	for name, value := range cmDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	if a.SingleNode {
		args["leader-elect"] = "false"
	}

	args = clusterConfig.Spec.FeatureGates.BuildArgs(args, kubeControllerManagerComponent)

	if args.Equals(a.previousConfig) && a.supervisor != nil {
		// no changes and supervisor already running, do nothing
		logger.Info("reconcile has nothing to do")
		return nil
	}
	// Stop in case there's process running already and we need to change the config
	if a.supervisor != nil {
		logger.Info("reconcile has nothing to do")
		err := a.supervisor.Stop()
		a.supervisor = nil
		if err != nil {
			return err
		}
	}

	a.supervisor = &supervisor.Supervisor{
		Name:    kubeControllerManagerComponent,
		BinPath: assets.BinPath(kubeControllerManagerComponent, a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    args.ToDashedArgs(),
		UID:     a.uid,
		GID:     a.gid,
	}
	a.previousConfig = args
	return a.supervisor.Supervise()
}

// Stop stops Manager
func (a *Manager) Stop() error {
	if a.supervisor != nil {
		return a.supervisor.Stop()
	}
	return nil
}
