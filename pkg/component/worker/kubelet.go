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

package worker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	NodeName            apitypes.NodeName
	CRISocket           string
	EnableCloudProvider bool
	K0sVars             *config.CfgVars
	Kubeconfig          string
	Configuration       kubeletv1beta1.KubeletConfiguration
	StaticPods          StaticPods
	LogLevel            string
	ClusterDNS          string
	Labels              []string
	Taints              []string
	ExtraArgs           stringmap.StringMap
	DualStackEnabled    bool

	configPath string
	supervisor supervisor.Supervisor
}

var _ manager.Component = (*Kubelet)(nil)

// Init extracts the needed binaries
func (k *Kubelet) Init(_ context.Context) error {

	if runtime.GOOS == "windows" {
		err := assets.Stage(k.K0sVars.BinDir, "kubelet.exe")
		return err
	}

	if runtime.GOOS == "linux" {
		if err := assets.Stage(k.K0sVars.BinDir, "kubelet"); err != nil {
			return err
		}
	}

	err := dir.Init(k.K0sVars.KubeletRootDir, constant.DataDirMode)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", k.K0sVars.KubeletRootDir, err)
	}

	runDir := filepath.Join(k.K0sVars.RunDir, "kubelet")
	if err := dir.Init(runDir, constant.RunDirMode); err != nil {
		return fmt.Errorf("failed to create %s: %w", runDir, err)
	}
	k.configPath = filepath.Join(runDir, "config.yaml")
	// Delete legacy config file (removed in 1.32)
	if err := os.Remove(filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		logrus.WithError(err).Warn("Failed to remove legacy kubelet config file")
	}

	return nil
}

func lookupNodeName(ctx context.Context, nodeName apitypes.NodeName) (ipv4 net.IP, ipv6 net.IP, _ error) {
	ipaddrs, err := net.DefaultResolver.LookupIPAddr(ctx, string(nodeName))
	if err != nil {
		return nil, nil, err
	}

	for _, addr := range ipaddrs {
		if ipv4 == nil && addr.IP.To4() != nil {
			ipv4 = addr.IP
		} else if ipv6 == nil && addr.IP.To16() != nil && addr.IP.To4() == nil {
			ipv6 = addr.IP
		}

		if ipv4 != nil && ipv6 != nil {
			break
		}
	}
	return ipv4, ipv6, nil
}

// Run runs kubelet
func (k *Kubelet) Start(ctx context.Context) error {
	cmd := "kubelet"

	if runtime.GOOS == "windows" {
		cmd = "kubelet.exe"
	}

	logrus.Info("Starting kubelet")
	args := stringmap.StringMap{
		"--root-dir":        k.K0sVars.KubeletRootDir,
		"--config":          k.configPath,
		"--kubeconfig":      k.Kubeconfig,
		"--v":               k.LogLevel,
		"--runtime-cgroups": "/system.slice/containerd.service",
		"--cert-dir":        filepath.Join(k.K0sVars.KubeletRootDir, "pki"),
	}

	if len(k.Labels) > 0 {
		args["--node-labels"] = strings.Join(k.Labels, ",")
	}

	if k.DualStackEnabled && k.ExtraArgs["--node-ip"] == "" {
		// Kubelet uses a DNS lookup of the node name to figure out the node IP,
		// but will only pick one for a single family. Do something similar as
		// kubelet, but for both IPv4 and IPv6.
		// https://github.com/kubernetes/kubernetes/blob/v1.32.1/pkg/kubelet/nodestatus/setters.go#L202-L230
		ipv4, ipv6, err := lookupNodeName(ctx, k.NodeName)
		if err != nil {
			logrus.WithError(err).Errorf("failed to lookup %q", k.NodeName)
		} else if ipv4 != nil && ipv6 != nil {
			// The kubelet will perform some extra validations on the discovered IP
			// addresses in the private function k8s.io/kubernetes/pkg/kubelet.validateNodeIP
			// which won't be replicated here.
			args["--node-ip"] = ipv4.String() + "," + ipv6.String()
		}
	}

	if runtime.GOOS == "windows" {
		args["--enforce-node-allocatable"] = ""
		args["--hairpin-mode"] = "promiscuous-bridge"
		args["--cert-dir"] = "C:\\var\\lib\\k0s\\kubelet_certs"
	}

	if k.CRISocket == "" && runtime.GOOS != "windows" {
		// on windows this cli flag is not supported
		// Still use this deprecated cAdvisor flag that the kubelet leaks until
		// KEP 2371 lands. ("cAdvisor-less, CRI-full Container and Pod Stats")
		args["--containerd"] = filepath.Join(k.K0sVars.RunDir, "containerd.sock")
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args["--cloud-provider"] = "external"
	}

	// Handle the extra args as last so they can be used to override some k0s "hardcodings"
	args.Merge(k.ExtraArgs)

	// Pin the node name that has been figured out by k0s
	args["--hostname-override"] = string(k.NodeName)

	logrus.Debugf("starting kubelet with args: %v", args)
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args.ToArgs(),
	}

	if err := k.writeKubeletConfig(); err != nil {
		return err
	}

	return k.supervisor.Supervise()
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	k.supervisor.Stop()
	return nil
}

func (k *Kubelet) writeKubeletConfig() error {
	var staticPodURL string
	if k.StaticPods != nil {
		url, err := k.StaticPods.ManifestURL()
		if err != nil {
			return err
		}
		staticPodURL = url.String()
	}

	containerRuntimeEndpoint, err := GetContainerRuntimeEndpoint(k.CRISocket, k.K0sVars.RunDir)
	if err != nil {
		return err
	}

	caPath, err := k.getKubeletCAPath()
	if err != nil {
		return err
	}

	config := k.Configuration.DeepCopy()
	config.Authentication.X509.ClientCAFile = caPath
	config.VolumePluginDir = k.K0sVars.KubeletVolumePluginDir
	config.ResolverConfig = determineKubeletResolvConfPath()
	config.StaticPodURL = staticPodURL
	config.ContainerRuntimeEndpoint = containerRuntimeEndpoint.String()

	if len(k.Taints) > 0 {
		var taints []corev1.Taint
		for _, taint := range k.Taints {
			parsedTaint, err := parseTaint(taint)
			if err != nil {
				return fmt.Errorf("can't parse taints for profile config map: %w", err)
			}
			taints = append(taints, parsedTaint)
		}
		config.RegisterWithTaints = taints
	}

	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("can't marshal kubelet config: %w", err)
	}

	err = file.WriteContentAtomically(k.configPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	return nil
}

func (k *Kubelet) getKubeletCAPath() (string, error) {
	restConfig, err := kubernetes.ClientConfig(kubernetes.KubeconfigFromFile(k.Kubeconfig))
	if err != nil {
		return "", fmt.Errorf("failed to load kubelet kubeconfig: %w", err)
	}

	if len(restConfig.CAData) > 0 {
		caPath := filepath.Join(k.K0sVars.RunDir, "kubelet", "ca.crt")
		if err := file.WriteContentAtomically(caPath, restConfig.CAData, constant.CertMode); err != nil {
			return "", fmt.Errorf("failed to write kubelet CA file: %w", err)
		}
		return caPath, nil
	}

	if !file.Exists(restConfig.CAFile) {
		return "", fmt.Errorf("kubelet CA file doesn't exist: %s", restConfig.CAFile)
	}

	return restConfig.CAFile, nil
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
			return taint, fmt.Errorf("invalid taint spec: %s", st)
		}
		key = partsKV[0]
		if len(partsKV) == 2 {
			value = partsKV[1]
			if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
				return taint, fmt.Errorf("invalid taint spec: %s, %s", st, strings.Join(errs, "; "))
			}
		}
	default:
		return taint, fmt.Errorf("invalid taint spec: %s", st)
	}

	if errs := validation.IsQualifiedName(key); len(errs) > 0 {
		return taint, fmt.Errorf("invalid taint spec: %s, %s", st, strings.Join(errs, "; "))
	}

	taint.Key = key
	taint.Value = value
	taint.Effect = effect

	return taint, nil
}

func validateTaintEffect(effect corev1.TaintEffect) error {
	if effect != corev1.TaintEffectNoSchedule && effect != corev1.TaintEffectPreferNoSchedule && effect != corev1.TaintEffectNoExecute {
		return fmt.Errorf("invalid taint effect: %s, unsupported taint effect", effect)
	}

	return nil
}

// determineKubeletResolvConfPath returns the path to the resolv.conf file that
// the kubelet should use.
func determineKubeletResolvConfPath() *string {
	path := "/etc/resolv.conf"

	switch runtime.GOOS {
	case "windows":
		return nil

	case "linux":
		// https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html#/etc/resolv.conf
		// If it's likely that resolv.conf is pointing to a systemd-resolved
		// nameserver, that nameserver won't be reachable from within
		// containers. Try to use the alternative resolv.conf path used by
		// systemd-resolved instead.
		detected, err := hasSystemdResolvedNameserver(path)
		if err != nil {
			logrus.WithError(err).Info("Failed to detect the presence of systemd-resolved")
		} else if detected {
			systemdPath := "/run/systemd/resolve/resolv.conf"
			logrus.Infof("The file %s looks like it's managed by systemd-resolved, using resolv.conf: %s", path, systemdPath)
			return &systemdPath
		}
	}

	logrus.Infof("Using resolv.conf: %s", path)
	return &path
}

// hasSystemdResolvedNameserver parses the given resolv.conf file and checks if
// it contains 127.0.0.53 as the only nameserver. Then it is assumed to be
// systemd-resolved managed.
func hasSystemdResolvedNameserver(resolvConfPath string) (bool, error) {
	f, err := os.Open(resolvConfPath)
	if err != nil {
		return false, err
	}

	defer f.Close()

	// This is roughly how glibc and musl do it: check for "nameserver" followed
	// by whitespace, then try to parse the next bytes as IP address,
	// disregarding anything after any additional whitespace.
	// https://sourceware.org/git/?p=glibc.git;a=blob;f=resolv/res_init.c;h=cce842fa9311c5bdba629f5e78c19746f75ef18e;hb=refs/tags/glibc-2.37#l396
	// https://git.musl-libc.org/cgit/musl/tree/src/network/resolvconf.c?h=v1.2.3#n62

	nameserverLine := regexp.MustCompile(`^nameserver\s+(\S+)`)

	lines := bufio.NewScanner(f)
	systemdResolvedIPSeen := false
	for lines.Scan() {
		match := nameserverLine.FindSubmatch(lines.Bytes())
		if len(match) < 1 {
			continue
		}
		ip := net.ParseIP(string(match[1]))
		if ip == nil {
			continue
		}
		if systemdResolvedIPSeen || !ip.Equal(net.IP{127, 0, 0, 53}) {
			return false, nil
		}
		systemdResolvedIPSeen = true
	}
	if err := lines.Err(); err != nil {
		return false, err
	}

	return systemdResolvedIPSeen, nil
}
