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
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Manager implement the component interface to run kube scheduler
type Manager struct {
	ClusterConfig *config.ClusterConfig
	gid           int
	K0sVars       constant.CfgVars
	LogLevel      string
	supervisor    supervisor.Supervisor
	uid           int
}

var cmDefaultArgs = map[string]string{
	"allocate-node-cidrs":             "true",
	"bind-address":                    "127.0.0.1",
	"cluster-name":                    "k0s",
	"controllers":                     "*,bootstrapsigner,tokencleaner",
	"enable-hostpath-provisioner":     "true",
	"leader-elect":                    "true",
	"use-service-account-credentials": "true",
}

// Init extracts the needed binaries
func (a *Manager) Init() error {
	var err error
	// controller manager running as api-server user as they both need access to same sa.key
	a.uid, err = util.GetUID(constant.ApiserverUser)
	if err != nil {
		logrus.Warning(fmt.Errorf("Running kube-controller-manager as root: %w", err))
	}

	// controller manager should be the only component that needs access to
	// ca.key so let it own it.
	if err := os.Chown(path.Join(a.K0sVars.CertRootDir, "ca.key"), a.uid, -1); err != nil && os.Geteuid() == 0 {
		logrus.Warning(fmt.Errorf("Can't change permissions for the ca.key: %w", err))
	}
	return assets.Stage(a.K0sVars.BinDir, "kube-controller-manager", constant.BinDirMode)
}

// Run runs kube Manager
func (a *Manager) Run() error {
	logrus.Info("Starting kube-controller-manager")
	ccmAuthConf := filepath.Join(a.K0sVars.CertRootDir, "ccm.conf")
	args := map[string]string{
		"authentication-kubeconfig":        ccmAuthConf,
		"authorization-kubeconfig":         ccmAuthConf,
		"kubeconfig":                       ccmAuthConf,
		"client-ca-file":                   path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"cluster-signing-cert-file":        path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"cluster-signing-key-file":         path.Join(a.K0sVars.CertRootDir, "ca.key"),
		"requestheader-client-ca-file":     path.Join(a.K0sVars.CertRootDir, "front-proxy-ca.crt"),
		"root-ca-file":                     path.Join(a.K0sVars.CertRootDir, "ca.crt"),
		"service-account-private-key-file": path.Join(a.K0sVars.CertRootDir, "sa.key"),
		"cluster-cidr":                     a.ClusterConfig.Spec.Network.BuildPodCIDR(),
		"service-cluster-ip-range":         a.ClusterConfig.Spec.Network.BuildServiceCIDR(a.ClusterConfig.Spec.API.Address),
		"profiling":                        "false",
		"terminated-pod-gc-threshold":      "12500",
		"v":                                a.LogLevel,
	}

	for name, value := range a.ClusterConfig.Spec.ControllerManager.ExtraArgs {
		if args[name] != "" && name != "profiling" {
			return fmt.Errorf("cannot override kube-controller-manager flag: %s", name)
		}
		args[name] = value
	}
	if a.ClusterConfig.Spec.Network.DualStack.Enabled {
		args["node-cidr-mask-size-ipv6"] = "110"
		args["node-cidr-mask-size-ipv4"] = "24"
	} else {
		args["node-cidr-mask-size"] = "24"
	}
	a.ClusterConfig.Spec.Network.DualStack.EnableDualStackFeatureGate(args)
	for name, value := range cmDefaultArgs {
		if args[name] == "" {
			args[name] = value
		}
	}
	cmArgs := []string{}
	for name, value := range args {
		cmArgs = append(cmArgs, fmt.Sprintf("--%s=%s", name, value))
	}

	if a.ClusterConfig.Spec.API.ExternalAddress == "" {
		cmArgs = append(cmArgs, "--leader-elect=false")
	}

	a.supervisor = supervisor.Supervisor{
		Name:    "kube-controller-manager",
		BinPath: assets.BinPath("kube-controller-manager", a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    cmArgs,
		UID:     a.uid,
		GID:     a.gid,
	}

	return a.supervisor.Supervise()
}

// Stop stops Manager
func (a *Manager) Stop() error {
	return a.supervisor.Stop()
}

// Health-check interface
func (a *Manager) Healthy() error { return nil }
