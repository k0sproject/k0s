// +build windows

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
package worker

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	CRISocket           string
	EnableCloudProvider bool
	K0sVars             constant.CfgVars
	KubeletConfigClient *KubeletConfigClient
	LogLevel            string
	Profile             string
	dataDir             string
	supervisor          supervisor.Supervisor
	ClusterDNS          string
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	cmd := "kubelet.exe"
	err := assets.Stage(k.K0sVars.BinDir, cmd, constant.BinDirMode)
	if err != nil {
		return err
	}

	k.dataDir = filepath.Join(k.K0sVars.DataDir, "kubelet")
	err = util.InitDirectory(k.dataDir, constant.DataDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.dataDir)
	}

	return nil
}

// Run runs kubelet
func (k *Kubelet) Run() error {
	cmd := "kubelet.exe"

	logrus.Info("Starting kubelet")
	kubeletConfigPath := filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")

	node, err := getNodeName()
	if err != nil {
		return fmt.Errorf("can't get hostname: %v", err)
	}

	args := util.MappedArgs{
		"--root-dir":                  k.dataDir,
		"--config":                    kubeletConfigPath,
		"--bootstrap-kubeconfig":      k.K0sVars.KubeletBootstrapConfigPath,
		"--kubeconfig":                k.K0sVars.KubeletAuthConfigPath,
		"--v":                         k.LogLevel,
		"--pod-infra-container-image": "mcr.microsoft.com/oss/kubernetes/pause:1.4.1",
		"--network-plugin":            "cni",
		"--cni-bin-dir":               "C:\\k\\cni",
		"--cni-conf-dir":              "C:\\k\\cni\\config",
		"--hostname-override":         node,
		"--cluster-domain":            "cluster.local",
		"--hairpin-mode":              "promiscuous-bridge",
		"--cert-dir":                  "C:\\var\\lib\\k0s\\kubelet_certs",
		"--cgroups-per-qos":           "false",
		"--enforce-node-allocatable":  "",
		"--resolv-conf":               "",
	}

	if k.CRISocket != "" {
		rtType, rtSock, err := splitRuntimeConfig(k.CRISocket)
		if err != nil {
			return err
		}
		args["--container-runtime"] = rtType
		shimPath := "npipe:////./pipe/dockershim"

		if rtType == "docker" {
			args["--docker-endpoint"] = rtSock
			// this endpoint is actually pointing to the one kubelet itself creates as the cri shim between itself and docker
			args["--container-runtime-endpoint"] = shimPath
		} else {
			args["--container-runtime-endpoint"] = rtSock
		}
	} else {
		sockPath := "npipe:////.//pipe//containerd-containerd"
		args["--container-runtime"] = "remote"
		args["--container-runtime-endpoint"] = sockPath
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args["--cloud-provider"] = "external"
	}
	argList := args.ToArgs()
	logrus.Infof("starting kubelet with args: %v", argList)
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    argList,
	}

	err = retry.Do(func() error {
		kubeletconfig, err := k.KubeletConfigClient.Get(k.Profile)
		if err != nil {
			logrus.Warnf("failed to get initial kubelet config with join token: %s", err.Error())
			return err
		}

		err = ioutil.WriteFile(kubeletConfigPath, []byte(kubeletconfig), constant.CertSecureMode)
		if err != nil {
			return errors.Wrap(err, "failed to write kubelet config to disk")
		}

		return nil
	})
	if err != nil {
		return err
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}

// Health-check interface
func (k *Kubelet) Healthy() error { return nil }
