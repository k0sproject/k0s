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
package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
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
	Labels              []string
	ExtraArgs           string
}

type kubeletConfig struct {
	ClientCAFile       string
	VolumePluginDir    string
	KubeReservedCgroup string
	KubeletCgroups     string
	CgroupsPerQOS      bool
	ResolvConf         string
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	cmds := []string{"kubelet", "xtables-legacy-multi"}

	if runtime.GOOS == "windows" {
		cmds = []string{"kubelet.exe"}
	}

	for _, cmd := range cmds {
		err := assets.Stage(k.K0sVars.BinDir, cmd, constant.BinDirMode)
		if err != nil {
			return err
		}
	}

	if runtime.GOOS == "linux" {
		for _, symlink := range []string{"iptables-save", "iptables-restore", "ip6tables", "ip6tables-save", "ip6tables-restore"} {
			symlinkPath := filepath.Join(k.K0sVars.BinDir, symlink)

			// remove if it exist and ignore error if it doesn't
			_ = os.Remove(symlinkPath)

			err := os.Symlink("xtables-legacy-multi", symlinkPath)
			if err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", symlink, err)
			}
		}
	}

	k.dataDir = filepath.Join(k.K0sVars.DataDir, "kubelet")
	err := dir.Init(k.dataDir, constant.DataDirMode)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", k.dataDir, err)
	}

	return nil
}

// Run runs kubelet
func (k *Kubelet) Run(ctx context.Context) error {
	cmd := "kubelet"

	kubeletConfigData := kubeletConfig{
		ClientCAFile:       filepath.Join(k.K0sVars.CertRootDir, "ca.crt"),
		VolumePluginDir:    k.K0sVars.KubeletVolumePluginDir,
		KubeReservedCgroup: "system.slice",
		KubeletCgroups:     "/system.slice/containerd.service",
	}
	if runtime.GOOS == "windows" {
		cmd = "kubelet.exe"
	}

	logrus.Info("Starting kubelet")
	kubeletConfigPath := filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")
	// get the "real" resolv.conf file (in systemd-resolvd bases system,
	// this will return /run/systemd/resolve/resolv.conf
	resolvConfPath := resolvconf.Path()

	args := stringmap.StringMap{
		"--root-dir":             k.dataDir,
		"--config":               kubeletConfigPath,
		"--bootstrap-kubeconfig": k.K0sVars.KubeletBootstrapConfigPath,
		"--kubeconfig":           k.K0sVars.KubeletAuthConfigPath,
		"--v":                    k.LogLevel,
		"--runtime-cgroups":      "/system.slice/containerd.service",
		"--cert-dir":             filepath.Join(k.dataDir, "pki"),
	}

	if len(k.Labels) > 0 {
		args["--node-labels"] = strings.Join(k.Labels, ",")
	}

	if runtime.GOOS == "windows" {
		node, err := getNodeName(ctx)
		if err != nil {
			return fmt.Errorf("can't get hostname: %v", err)
		}
		kubeletConfigData.CgroupsPerQOS = false
		kubeletConfigData.ResolvConf = ""
		args["--enforce-node-allocatable"] = ""
		args["--pod-infra-container-image"] = "mcr.microsoft.com/oss/kubernetes/pause:1.4.1"
		args["--network-plugin"] = "cni"
		args["--cni-bin-dir"] = "C:\\k\\cni"
		args["--cni-conf-dir"] = "C:\\k\\cni\\config"
		args["--hostname-override"] = node
		args["--cluster-domain"] = "cluster.local"
		args["--hairpin-mode"] = "promiscuous-bridge"
		args["--cert-dir"] = "C:\\var\\lib\\k0s\\kubelet_certs"
	} else {
		kubeletConfigData.CgroupsPerQOS = true
		kubeletConfigData.ResolvConf = resolvConfPath
	}

	if k.CRISocket != "" {
		rtType, rtSock, err := SplitRuntimeConfig(k.CRISocket)
		if err != nil {
			return err
		}
		args["--container-runtime"] = rtType
		shimPath := "unix:///var/run/dockershim.sock"
		if runtime.GOOS == "windows" {
			shimPath = "npipe:////./pipe/dockershim"
		}
		if rtType == "docker" {
			args["--docker-endpoint"] = rtSock
			// this endpoint is actually pointing to the one kubelet itself creates as the cri shim between itself and docker
			args["--container-runtime-endpoint"] = shimPath
		} else {
			args["--container-runtime-endpoint"] = rtSock
		}
	} else {
		sockPath := path.Join(k.K0sVars.RunDir, "containerd.sock")
		args["--container-runtime"] = "remote"
		args["--container-runtime-endpoint"] = fmt.Sprintf("unix://%s", sockPath)
		args["--containerd"] = sockPath
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args["--cloud-provider"] = "external"
	}

	// Handle the extra args as last so they can be used to overrride some k0s "hardcodings"
	if k.ExtraArgs != "" {
		extras := flags.Split(k.ExtraArgs)
		args.Merge(extras)
	}

	logrus.Infof("starting kubelet with args: %v", args)
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args.ToArgs(),
	}

	err := retry.Do(func() error {
		kubeletconfig, err := k.KubeletConfigClient.Get(ctx, k.Profile)
		if err != nil {
			logrus.Warnf("failed to get initial kubelet config with join token: %s", err.Error())
			return err
		}
		tw := templatewriter.TemplateWriter{
			Name:     "kubelet-config",
			Template: kubeletconfig,
			Data:     kubeletConfigData,
			Path:     kubeletConfigPath,
		}
		err = tw.Write()
		if err != nil {
			return fmt.Errorf("failed to write kubelet config: %w", err)
		}

		return nil
	},
		retry.Delay(time.Millisecond*500),
		retry.DelayType(retry.BackOffDelay))
	if err != nil {
		return err
	}

	return k.supervisor.Supervise()
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}

// Reconcile detects changes in configuration and applies them to the component
func (k *Kubelet) Reconcile() error {
	logrus.Debug("reconcile method called for: Kubelet")
	return nil
}

// Health-check interface
func (k *Kubelet) Healthy() error { return nil }

const awsMetaInformationURI = "http://169.254.169.254/latest/meta-data/local-hostname"

func getNodeName(ctx context.Context) (string, error) {
	req, err := http.NewRequest("GET", awsMetaInformationURI, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return os.Hostname()
	}
	defer resp.Body.Close()
	h, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("can't read aws hostname: %v", err)
	}
	return string(h), nil
}
