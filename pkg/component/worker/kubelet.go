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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
    "strconv"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/internal/pkg/iptablesutils"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/node"
	"github.com/k0sproject/k0s/pkg/supervisor"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	CRISocket           string
	EnableCloudProvider bool
	K0sVars             *config.CfgVars
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
	DualStackEnabled    bool
}

var _ manager.Component = (*Kubelet)(nil)

type kubeletConfig struct {
	ClientCAFile       string
	VolumePluginDir    string
    ReservedSystemCPUs string
    SystemReservedCPU  string
    SystemReservedMemory string

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

func lookupHostname(ctx context.Context, hostname string) (ipv4 net.IP, ipv6 net.IP, _ error) {
	ipaddrs, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
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

// Get cgroup path
func getCgroup() string {
	file, err := os.Open("/proc/1/cgroup")
	if err != nil {
		fmt.Println("Error opening /proc/1/cgroup:", err)
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) == 3 {
			return filepath.Join("/sys/fs/cgroup", fields[2])
		}
	}
	return ""
}

// Get memory limit from cgroup
func getMemory() int64 {
	cgroupPath := getCgroup()
	memoryFile := filepath.Join(cgroupPath, "memory.max")
	content, err := os.ReadFile(memoryFile)
	if err != nil {
		fmt.Println("Error reading memory.max file:", err)
		return 0
	}

	memoryStr := strings.TrimSpace(string(content))
	if memoryStr == "max" {
		return 0
	}

	memory, err := strconv.ParseInt(memoryStr, 10, 64)
	if err != nil {
		fmt.Println("Error parsing memory:", err)
		return 0
	}

	return memory
}

// Get CPU list from cgroup
func getCpus() string {
	cgroupPath := getCgroup()
	cpusFile := filepath.Join(cgroupPath, "cpuset.cpus")
	content, err := os.ReadFile(cpusFile)
	if err != nil {
		fmt.Println("Error reading cpuset.cpus file:", err)
		return ""
	}

	return strings.TrimSpace(string(content))
}

// Get CPU quota from cgroup
func getCpuQuota() int {
    cgroupPath := getCgroup()
    cpumaxFile := filepath.Join(cgroupPath, "cpu.max")
    content, err := os.ReadFile(cpumaxFile)
    if err != nil {
        fmt.Println("Error reading cpu.max file:", err)
        return 100000
    }

    // parse a string of the form max 100000
    parts := strings.Fields(string(content))
    if len(parts) != 2 {
        fmt.Println("Error parsing cpu.max:", err)
        return 100000
    }

    if parts[0] == "max" {
        return 100000
    }

    quota, err := strconv.Atoi(parts[0])
    if err != nil {
        fmt.Println("Error parsing cpu.max:", err)
        return 100000
    }

    return quota
}


// Get total system memory
func getMemTotal() int64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		fmt.Println("Error opening /proc/meminfo:", err)
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal") {
			parts := strings.Fields(line)
			memTotalKb, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				fmt.Println("Error parsing memory total:", err)
				return 0
			}
			return memTotalKb * 1024 // Convert KB to bytes
		}
	}
	return 0
}

// Invert memory usage
func invertMemoryUsage(memory int64) int64 {
	if memory == 0 {
		return 0
	}
	return getMemTotal() - memory
}

// Count number of processors
func numProcessors() int {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		fmt.Println("Error opening /proc/cpuinfo:", err)
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	processorCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			processorCount++
		}
	}
	return processorCount
}

// Build CPU list as a boolean array
func buildCpuList(cpu string, ncpu int) []bool {
	cpuPresenceArray := make([]bool, ncpu)
	cpuRanges := strings.Split(cpu, ",")

	for _, item := range cpuRanges {
		if strings.Contains(item, "-") {
			rangeParts := strings.Split(item, "-")
			start, _ := strconv.Atoi(rangeParts[0])
			end, _ := strconv.Atoi(rangeParts[1])

			for i := start; i <= end && i < ncpu; i++ {
				cpuPresenceArray[i] = true
			}
		} else {
			cpuIndex, _ := strconv.Atoi(strings.TrimSpace(item))
			if cpuIndex < ncpu {
				cpuPresenceArray[cpuIndex] = true
			}
		}
	}

	return cpuPresenceArray
}

// Build CPU string from boolean array
func buildCpuString(cpus []bool) string {
	var cpuList []string
	i := 0
	for i < len(cpus) {
		if cpus[i] {
			start := i
			for i < len(cpus) && cpus[i] {
				i++
			}
			end := i - 1
			if start == end {
				cpuList = append(cpuList, fmt.Sprintf("%d", start))
			} else {
				cpuList = append(cpuList, fmt.Sprintf("%d-%d", start, end))
			}
		}
		i++
	}
	return strings.Join(cpuList, ",")
}

// Invert CPU list
func invertCpuList(cpu string) string {
	if cpu == "" {
		return ""
	}

	ncpu := numProcessors()
	cpuList := buildCpuList(cpu, ncpu)
	for i := 0; i < ncpu; i++ {
		cpuList[i] = !cpuList[i]
	}

	return buildCpuString(cpuList)
}

func buildCpuQuotaString(quota int) string {
    return fmt.Sprintf("%dm", (100000-quota)/100)
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
		ClientCAFile:           filepath.Join(k.K0sVars.CertRootDir, "ca.crt"),
		VolumePluginDir:        k.K0sVars.KubeletVolumePluginDir,
        ReservedSystemCPUs:     invertCpuList(getCpus()),
        SystemReservedMemory:   strconv.FormatInt(invertMemoryUsage(getMemory()), 10),
        SystemReservedCPU:      buildCpuQuotaString(getCpuQuota()),
		StaticPodURL:           staticPodURL,
	}
	if runtime.GOOS == "windows" {
		cmd = "kubelet.exe"
	}

	logrus.Info("Starting kubelet")
	kubeletConfigPath := filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")

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

	extras := flags.Split(k.ExtraArgs)
	nodename, err := node.GetNodename(extras["--hostname-override"])
	if err != nil {
		return fmt.Errorf("failed to get nodename: %w", err)
	}

	if k.DualStackEnabled && extras["--node-ip"] == "" {
		// Kubelet uses hostname lookup to autodetect the ip address, but
		// will only pick one for a single family. Do something similar as
		// kubelet but for both ipv6 and ipv6.
		// https://github.com/kubernetes/kubernetes/blob/0cc57258c3f8545c8250f0e7a1307fd01b0d283d/pkg/kubelet/nodestatus/setters.go#L196
		ipv4, ipv6, err := lookupHostname(ctx, nodename)
		if err != nil {
			logrus.WithError(err).Errorf("failed to lookup %q", nodename)
		} else if ipv4 != nil && ipv6 != nil {
			args["--node-ip"] = ipv4.String() + "," + ipv6.String()
		}
	}

	if runtime.GOOS == "windows" {
		kubeletConfigData.CgroupsPerQOS = false
		kubeletConfigData.ResolvConf = ""
		args["--enforce-node-allocatable"] = ""
		args["--hostname-override"] = nodename
		args["--hairpin-mode"] = "promiscuous-bridge"
		args["--cert-dir"] = "C:\\var\\lib\\k0s\\kubelet_certs"
	} else {
		kubeletConfigData.CgroupsPerQOS = true
		kubeletConfigData.ResolvConf = determineKubeletResolvConfPath()
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
	if k.ExtraArgs != "" {
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
	k.supervisor.Stop()
	return nil
}

func (k *Kubelet) prepareLocalKubeletConfig(kubeletConfigData kubeletConfig) (string, error) {
	preparedConfig := k.Configuration.DeepCopy()
	preparedConfig.Authentication.X509.ClientCAFile = kubeletConfigData.ClientCAFile // filepath.Join(k.K0sVars.CertRootDir, "ca.crt")
	preparedConfig.VolumePluginDir = kubeletConfigData.VolumePluginDir               // k.K0sVars.KubeletVolumePluginDir
    preparedConfig.ReservedSystemCPUs = kubeletConfigData.ReservedSystemCPUs
    preparedConfig.SystemReserved = make(map[string]string)
    preparedConfig.SystemReserved["memory"] = kubeletConfigData.SystemReservedMemory
    preparedConfig.SystemReserved["cpu"] = kubeletConfigData.SystemReservedCPU
	preparedConfig.ResolverConfig = ptr.To(kubeletConfigData.ResolvConf)
	preparedConfig.CgroupsPerQOS = ptr.To(kubeletConfigData.CgroupsPerQOS)
	preparedConfig.StaticPodURL = kubeletConfigData.StaticPodURL

	containerRuntimeEndpoint, err := GetContainerRuntimeEndpoint(k.CRISocket, k.K0sVars.RunDir)
	if err != nil {
		return "", err
	}
	preparedConfig.ContainerRuntimeEndpoint = containerRuntimeEndpoint.String()

	if len(k.Taints) > 0 {
		var taints []corev1.Taint
		for _, taint := range k.Taints {
			parsedTaint, err := parseTaint(taint)
			if err != nil {
				return "", fmt.Errorf("can't parse taints for profile config map: %w", err)
			}
			taints = append(taints, parsedTaint)
		}
		preparedConfig.RegisterWithTaints = taints
	}

	preparedConfigBytes, err := yaml.Marshal(preparedConfig)
	if err != nil {
		return "", fmt.Errorf("can't marshal kubelet config: %w", err)
	}
	return string(preparedConfigBytes), nil
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
func determineKubeletResolvConfPath() string {
	path := "/etc/resolv.conf"

	// https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html#/etc/resolv.conf
	// If it's likely that resolv.conf is pointing to a systemd-resolved
	// nameserver, that nameserver won't be reachable from within containers.
	// Try to use the alternative resolv.conf path used by systemd-resolved instead.
	detected, err := hasSystemdResolvedNameserver(path)
	if err != nil {
		logrus.WithError(err).Infof("Error while trying to detect the presence of systemd-resolved, using resolv.conf: %s", path)
		return path
	}

	if detected {
		alternatePath := "/run/systemd/resolve/resolv.conf"
		logrus.Infof("The file %s looks like it's managed by systemd-resolved, using resolv.conf: %s", path, alternatePath)
		return alternatePath
	}

	logrus.Infof("Using resolv.conf: %s", path)
	return path
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
