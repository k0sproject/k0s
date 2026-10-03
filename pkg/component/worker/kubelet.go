// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/supervisor"

	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	cliflag "k8s.io/component-base/cli/flag"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	NodeName             apitypes.NodeName
	CRISocket            string
	EnableCloudProvider  bool
	K0sVars              *config.CfgVars
	Kubeconfig           string
	Configuration        kubeletv1beta1.KubeletConfiguration
	StaticPods           StaticPods
	LogLevel             string
	ClusterDNS           string
	Labels               map[string]string
	Taints               []string
	ExtraArgs            stringmap.StringMap
	DualStackEnabled     bool
	PrimaryAddressFamily v1beta1.PrimaryAddressFamilyType

	configPath     string
	supervisor     *supervisor.Supervisor
	executablePath string
}

var _ manager.Component = (*Kubelet)(nil)

// Init extracts the needed binaries
func (k *Kubelet) Init(_ context.Context) (err error) {
	if k.executablePath, err = assets.StageExecutable(k.K0sVars.BinDir, "kubelet"); err != nil {
		return err
	}

	if err = dir.Init(k.K0sVars.KubeletRootDir, constant.DataDirMode); err != nil {
		return fmt.Errorf("failed to create %s: %w", k.K0sVars.KubeletRootDir, err)
	}

	runDir := filepath.Join(k.K0sVars.RunDir, "kubelet")
	if err := dir.Init(runDir, constant.RunDirMode); err != nil {
		return fmt.Errorf("failed to create %s: %w", runDir, err)
	}
	k.configPath = filepath.Join(runDir, "config.yaml")

	return nil
}

func (k *Kubelet) lookupNodeName(ctx context.Context) (ipv4, ipv6 net.IP, _ error) {
	ipaddrs, err := net.DefaultResolver.LookupIPAddr(ctx, string(k.NodeName))
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
	logrus.Info("Starting kubelet")
	args := stringmap.StringMap{
		"--root-dir":   k.K0sVars.KubeletRootDir,
		"--config":     k.configPath,
		"--kubeconfig": k.Kubeconfig,
		"--v":          k.LogLevel,
		"--cert-dir":   filepath.Join(k.K0sVars.KubeletRootDir, "pki"),
	}

	if len(k.Labels) > 0 {
		args["--node-labels"] = ((*cliflag.ConfigurationMap)(&k.Labels)).String()
	}

	if k.DualStackEnabled && k.ExtraArgs["--node-ip"] == "" {
		// Kubelet uses a DNS lookup of the node name to figure out the node IP,
		// but will only pick one for a single family. Do something similar as
		// kubelet, but for both IPv4 and IPv6.
		// https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/kubelet/nodestatus/setters.go#L151-L179
		ipv4, ipv6, err := k.lookupNodeName(ctx)
		if err == nil && (ipv4 == nil || ipv6 == nil) {
			err = fmt.Errorf("node name IP address lookup didn't return addresses for both families: IPv4: %s, IPv6: %s", ipv4, ipv6)
		}
		if err != nil {
			return fmt.Errorf("failed to detect node IPs for %q: %w", k.NodeName, err)
		}

		// The kubelet will perform some extra validations on the discovered IP
		// addresses in the private function k8s.io/kubernetes/pkg/kubelet.validateNodeIP
		// which won't be replicated here.
		if k.PrimaryAddressFamily == v1beta1.PrimaryFamilyIPv6 {
			args["--node-ip"] = ipv6.String() + "," + ipv4.String()
		} else {
			args["--node-ip"] = ipv4.String() + "," + ipv6.String()
		}
	}

	switch runtime.GOOS {
	case "linux":
		args["--runtime-cgroups"] = "/system.slice/containerd.service"

	case "windows":
		args["--enforce-node-allocatable"] = ""
		args["--hairpin-mode"] = "promiscuous-bridge"
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
	k.supervisor = &supervisor.Supervisor{
		Name:    "kubelet",
		BinPath: k.executablePath,
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args.ToArgs(),
	}

	if err := k.writeKubeletConfig(); err != nil {
		return err
	}

	return k.supervisor.Supervise(ctx)
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	if k.supervisor != nil {
		return k.supervisor.Stop()
	}
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
	if config.ResolverConfig == nil {
		if runtime.GOOS == "windows" {
			// https://github.com/kubernetes/kubernetes/issues/116782#issuecomment-1477536396
			config.ResolverConfig = new("")
		} else {
			path := resolvconf.Path
			if useUplink, err := useSystemdResolvedUplink("/"); err != nil {
				logrus.WithError(err).Warn("Failed to detect systemd-resolved")
			} else if useUplink {
				path = resolvconf.SystemdResolvedUplinkPath
			}
			logrus.Info("Using resolv.conf: ", path)
			config.ResolverConfig = &path
		}
	}
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

	err = file.WriteContentAtomically(k.configPath, configBytes, constant.OwnerOnlyMode)
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

// Determines if systemd-resolved's uplink resolv.conf should be used. If
// systemd-resolved is running, /etc/resolv.conf usually points to its stub
// resolver on localhost, which isn't reachable from within a pod's network
// namespace. If that's the case, systemd-resolved's uplink file should be used.
func useSystemdResolvedUplink(root string) (_ bool, err error) {
	uplinkInfo, err := os.Stat(filepath.Join(root, resolvconf.SystemdResolvedUplinkPath))
	if err != nil {
		logger := logrus.WithError(err)
		if errors.Is(err, os.ErrNotExist) {
			logger.Debug("Didn't detect systemd-resolved")
			return false, nil
		}
		return false, err
	}

	resolvConf, err := os.Open(filepath.Join(root, resolvconf.Path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Warn(resolvconf.Path, " doesn't exist")
			return true, nil
		}
		return false, err
	}
	defer func() { err = errors.Join(err, resolvConf.Close()) }()

	info, err := resolvConf.Stat()
	if err != nil {
		return false, err
	}

	if os.SameFile(info, uplinkInfo) {
		logrus.Debug(resolvconf.Path, " points to systemd-resolved's uplink")
		return false, nil
	}

	if isStub, err := resolvconf.IsSystemdResolvedStub(resolvConf); err != nil {
		if errors.Is(err, resolvconf.ErrNoNameservers) {
			logrus.Warn(resolvconf.Path, " has no nameservers, using systemd-resolved's uplink")
			return true, nil
		}
		return false, err
	} else if isStub {
		logrus.Info(resolvconf.Path, " points to systemd-resolved's stub, using its uplink instead")
		return true, nil
	}

	logrus.Debug(resolvconf.Path, " doesn't point to systemd-resolved's stub")
	return false, nil
}
