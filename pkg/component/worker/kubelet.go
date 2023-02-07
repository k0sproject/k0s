/*
Copyright 2023 k0s authors

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

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/internal/pkg/iptablesutils"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	"github.com/docker/libnetwork/resolvconf"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	CRISocket           string
	EnableCloudProvider bool
	K0sVars             constant.CfgVars
	Kubeconfig          string
	Configuration       kubeletv1beta1.KubeletConfiguration
	StaticPods          StaticPods
	LogLevel            string
	dataDir             string
	supervisor          supervisor.Supervisor
	ClusterDNS          string
	Labels              []string
	Taints              []string
	ExtraArgs           string
	IPTablesMode        string
}

var _ manager.Component = (*Kubelet)(nil)

type kubeletConfig struct {
	ClientCAFile       string
	VolumePluginDir    string
	KubeReservedCgroup string
	KubeletCgroups     string
	CgroupsPerQOS      bool
	ResolvConf         string
	StaticPodURL       string
}

// Init extracts the needed binaries
func (k *Kubelet) Init(_ context.Context) error {
	cmds := []string{"kubelet", "xtables-legacy-multi", "xtables-nft-multi"}

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
		iptablesMode := k.IPTablesMode
		if iptablesMode == "" || iptablesMode == "auto" {
			var err error
			iptablesMode, err = iptablesutils.DetectHostIPTablesMode(k.K0sVars.BinDir)
			if err != nil {
				if KernelMajorVersion() < 5 {
					iptablesMode = iptablesutils.ModeLegacy
				} else {
					iptablesMode = iptablesutils.ModeNFT
				}
				logrus.WithError(err).Infof("Failed to detect iptables mode, using iptables-%s by default", iptablesMode)
			}
		}
		logrus.Infof("using iptables-%s", iptablesMode)
		oldpath := fmt.Sprintf("xtables-%s-multi", iptablesMode)
		for _, symlink := range []string{"iptables", "iptables-save", "iptables-restore", "ip6tables", "ip6tables-save", "ip6tables-restore"} {
			symlinkPath := filepath.Join(k.K0sVars.BinDir, symlink)

			// remove if it exist and ignore error if it doesn't
			_ = os.Remove(symlinkPath)

			err := os.Symlink(oldpath, symlinkPath)
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
func (k *Kubelet) Start(ctx context.Context) error {
	cmd := "kubelet"

	var staticPodURL string
	if k.StaticPods != nil {
		var err error
		if staticPodURL, err = k.StaticPods.ManifestURL(); err != nil {
			return err
		}
	}

	kubeletConfigData := kubeletConfig{
		ClientCAFile:       filepath.Join(k.K0sVars.CertRootDir, "ca.crt"),
		VolumePluginDir:    k.K0sVars.KubeletVolumePluginDir,
		KubeReservedCgroup: "system.slice",
		KubeletCgroups:     "/system.slice/containerd.service",
		StaticPodURL:       staticPodURL,
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
		"--root-dir":        k.dataDir,
		"--config":          kubeletConfigPath,
		"--kubeconfig":      k.Kubeconfig,
		"--v":               k.LogLevel,
		"--runtime-cgroups": "/system.slice/containerd.service",
		"--cert-dir":        filepath.Join(k.dataDir, "pki"),
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
		args["--hairpin-mode"] = "promiscuous-bridge"
		args["--cert-dir"] = "C:\\var\\lib\\k0s\\kubelet_certs"
	} else {
		kubeletConfigData.CgroupsPerQOS = true
		kubeletConfigData.ResolvConf = resolvConfPath
	}

	if k.CRISocket != "" {
		// Due to the removal of dockershim from kube 1.24, we no longer need to
		// handle any special docker case
		_, rtSock, err := SplitRuntimeConfig(k.CRISocket)
		if err != nil {
			return err
		}
		args["--container-runtime-endpoint"] = rtSock

	} else {
		sockPath := path.Join(k.K0sVars.RunDir, "containerd.sock")
		args["--container-runtime-endpoint"] = fmt.Sprintf("unix://%s", sockPath)
		args["--containerd"] = sockPath
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args["--cloud-provider"] = "external"
	}

	// Handle the extra args as last so they can be used to override some k0s "hardcodings"
	if k.ExtraArgs != "" {
		extras := flags.Split(k.ExtraArgs)
		args.Merge(extras)
	}

	logrus.Debugf("starting kubelet with args: %v", args)
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args.ToArgs(),
	}

	kubeletconfig, err := k.prepareLocalKubeletConfig(kubeletConfigData)
	if err != nil {
		logrus.Warnf("failed to prepare local kubelet config: %s", err.Error())
		return err
	}
	err = file.WriteContentAtomically(kubeletConfigPath, []byte(kubeletconfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	return k.supervisor.Supervise()
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}

func (k *Kubelet) prepareLocalKubeletConfig(kubeletConfigData kubeletConfig) (string, error) {
	preparedConfig := k.Configuration.DeepCopy()
	preparedConfig.Authentication.X509.ClientCAFile = kubeletConfigData.ClientCAFile // filepath.Join(k.K0sVars.CertRootDir, "ca.crt")
	preparedConfig.VolumePluginDir = kubeletConfigData.VolumePluginDir               // k.K0sVars.KubeletVolumePluginDir
	preparedConfig.KubeReservedCgroup = kubeletConfigData.KubeReservedCgroup
	preparedConfig.KubeletCgroups = kubeletConfigData.KubeletCgroups
	preparedConfig.ResolverConfig = pointer.String(kubeletConfigData.ResolvConf)
	preparedConfig.CgroupsPerQOS = pointer.Bool(kubeletConfigData.CgroupsPerQOS)
	preparedConfig.StaticPodURL = kubeletConfigData.StaticPodURL

	if len(k.Taints) > 0 {
		var taints []corev1.Taint
		for _, taint := range k.Taints {
			parsedTaint, err := parseTaint(taint)
			if err != nil {
				return "", fmt.Errorf("can't parse taints for profile config map: %v", err)
			}
			taints = append(taints, parsedTaint)
		}
		preparedConfig.RegisterWithTaints = taints
	}

	preparedConfigBytes, err := yaml.Marshal(preparedConfig)
	if err != nil {
		return "", fmt.Errorf("can't marshal kubelet config: %v", err)
	}
	return string(preparedConfigBytes), nil
}

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

func parseTaint(st string) (corev1.Taint, error) {
	var taint corev1.Taint

	var key string
	var value string
	var effect corev1.TaintEffect

	parts := strings.Split(st, ":")
	switch len(parts) {
	case 1:
		key = parts[0]
	case 2:
		effect = corev1.TaintEffect(parts[1])
		if err := validateTaintEffect(effect); err != nil {
			return taint, err
		}

		partsKV := strings.Split(parts[0], "=")
		if len(partsKV) > 2 {
			return taint, fmt.Errorf("invalid taint spec: %v", st)
		}
		key = partsKV[0]
		if len(partsKV) == 2 {
			value = partsKV[1]
			if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
				return taint, fmt.Errorf("invalid taint spec: %v, %s", st, strings.Join(errs, "; "))
			}
		}
	default:
		return taint, fmt.Errorf("invalid taint spec: %v", st)
	}

	if errs := validation.IsQualifiedName(key); len(errs) > 0 {
		return taint, fmt.Errorf("invalid taint spec: %v, %s", st, strings.Join(errs, "; "))
	}

	taint.Key = key
	taint.Value = value
	taint.Effect = effect

	return taint, nil
}

func validateTaintEffect(effect corev1.TaintEffect) error {
	if effect != corev1.TaintEffectNoSchedule && effect != corev1.TaintEffectPreferNoSchedule && effect != corev1.TaintEffectNoExecute {
		return fmt.Errorf("invalid taint effect: %v, unsupported taint effect", effect)
	}

	return nil
}
