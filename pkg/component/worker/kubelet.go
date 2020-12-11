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
	"path"
	"path/filepath"
	"strings"

	"github.com/avast/retry-go"
	"github.com/docker/libnetwork/resolvconf"
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
}

// KubeletConfig defines the kubelet related config options
type KubeletConfig struct {
	ClusterDNS    string
	ClusterDomain string
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	err := assets.Stage(k.K0sVars.BinDir, "kubelet", constant.BinDirMode)
	if err != nil {
		return err
	}

	k.dataDir = filepath.Join(k.K0sVars.DataDir, "kubelet")
	err = util.InitDirectory(k.dataDir, constant.DataDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.dataDir)
	}

	err = util.InitDirectory(k.K0sVars.KubeletVolumePluginDir, constant.KubeletVolumePluginDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.K0sVars.KubeletVolumePluginDir)
	}

	return nil
}

// Run runs kubelet
func (k *Kubelet) Run() error {
	logrus.Info("Starting kubelet")
	kubeletConfigPath := filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")
	// get the "real" resolv.conf file (in systemd-resolvd bases system,
	// this will return /run/systemd/resolve/resolv.conf
	resolvConfPath := resolvconf.Path()

	args := []string{
		fmt.Sprintf("--root-dir=%s", k.dataDir),
		fmt.Sprintf("--volume-plugin-dir=%s", k.K0sVars.KubeletVolumePluginDir),

		fmt.Sprintf("--config=%s", kubeletConfigPath),
		fmt.Sprintf("--bootstrap-kubeconfig=%s", k.K0sVars.KubeletBootstrapConfigPath),
		fmt.Sprintf("--kubeconfig=%s", k.K0sVars.KubeletAuthConfigPath),
		fmt.Sprintf("--v=%s", k.LogLevel),
		fmt.Sprintf("--resolv-conf=%s", resolvConfPath),
		"--kube-reserved-cgroup=system.slice",
		"--runtime-cgroups=/system.slice/containerd.service",
		"--kubelet-cgroups=/system.slice/containerd.service",
	}

	if k.CRISocket != "" {
		rtType, rtSock, err := splitRuntimeConfig(k.CRISocket)
		if err != nil {
			return err
		}
		args = append(args, fmt.Sprintf("--container-runtime=%s", rtType))

		if rtType == "docker" {
			args = append(args, fmt.Sprintf("--docker-endpoint=%s", rtSock))
			// this endpoint is actually pointing to the one kubelet itself creates as the cri shim between itself and docker
			args = append(args, "--container-runtime-endpoint=unix:///var/run/dockershim.sock")
		} else {
			args = append(args, fmt.Sprintf("--container-runtime-endpoint=%s", rtSock))
		}
	} else {
		args = append(args, "--container-runtime=remote")
		args = append(args, fmt.Sprintf("--container-runtime-endpoint=unix://%s", path.Join(k.K0sVars.RunDir, "containerd.sock")))
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args = append(args, "--cloud-provider=external")
	}

	k.supervisor = supervisor.Supervisor{
		Name:    "kubelet",
		BinPath: assets.BinPath("kubelet", k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args,
	}

	err := retry.Do(func() error {
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

func splitRuntimeConfig(rtConfig string) (string, string, error) {
	runtimeConfig := strings.SplitN(rtConfig, ":", 2)
	if len(runtimeConfig) != 2 {
		return "", "", fmt.Errorf("cannot parse CRI socket path")
	}
	runtimeType := runtimeConfig[0]
	runtimeSocket := runtimeConfig[1]
	if runtimeType != "docker" && runtimeType != "remote" {
		return "", "", fmt.Errorf("unknown runtime type %s, must be either of remote or docker", runtimeType)
	}

	return runtimeType, runtimeSocket, nil
}
