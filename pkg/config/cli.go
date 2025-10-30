// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/featuregate"
	"github.com/k0sproject/k0s/pkg/k0scloudprovider"

	cliflag "k8s.io/component-base/cli/flag"

	"github.com/spf13/pflag"
)

var (
	CfgFile    string
	K0sVars    CfgVars
	workerOpts WorkerOptions
)

// This struct holds all the CLI options & settings required by the
// different k0s sub-commands
type CLIOptions struct {
	WorkerOptions
	CfgFile string
	K0sVars *CfgVars
}

type ControllerMode uint8

const (
	ControllerOnlyMode ControllerMode = iota
	ControllerPlusWorkerMode
	SingleNodeMode
)

// Shared controller cli flags
type ControllerOptions struct {
	NoTaints          bool
	DisableComponents []string
	InitOnly          bool

	ClusterComponents               *manager.Manager
	EnableK0sCloudProvider          bool
	K0sCloudProviderPort            int
	K0sCloudProviderUpdateFrequency time.Duration
	NodeComponents                  *manager.Manager
	EnableDynamicConfig             bool
	EnableMetricsScraper            bool
	KubeControllerManagerExtraArgs  string
	FeatureGates                    featuregate.FeatureGates

	enableWorker, singleNode bool
}

// Shared worker cli flags
type WorkerOptions struct {
	CloudProvider    bool
	LogLevels        LogLevels
	CriSocket        string
	KubeletExtraArgs string
	Labels           map[string]string
	Taints           []string
	TokenFile        string
	TokenArg         string
	WorkerProfile    string
	IPTablesMode     string
}

func (m ControllerMode) WorkloadsEnabled() bool {
	switch m {
	case ControllerPlusWorkerMode, SingleNodeMode:
		return true
	default:
		return false
	}
}

func (o *ControllerOptions) Mode() ControllerMode {
	switch {
	case o.singleNode:
		return SingleNodeMode
	case o.enableWorker:
		return ControllerPlusWorkerMode
	default:
		return ControllerOnlyMode
	}
}

func (o *ControllerOptions) Normalize() error {
	// Normalize component names
	var disabledComponents []string
	for _, disabledComponent := range o.DisableComponents {
		if !slices.Contains(availableComponents, disabledComponent) {
			return fmt.Errorf("unknown component %s", disabledComponent)
		}

		if !slices.Contains(disabledComponents, disabledComponent) {
			disabledComponents = append(disabledComponents, disabledComponent)
		}
	}
	o.DisableComponents = disabledComponents

	return nil
}

type LogLevels = struct {
	Containerd            string
	Etcd                  string
	Konnectivity          string
	KubeAPIServer         string
	KubeControllerManager string
	KubeScheduler         string
	Kubelet               string
}

func DefaultLogLevels() LogLevels {
	return LogLevels{
		Containerd:            "info",
		Etcd:                  "info",
		Konnectivity:          "1",
		KubeAPIServer:         "1",
		KubeControllerManager: "1",
		KubeScheduler:         "1",
		Kubelet:               "1",
	}
}

type logLevelsFlag LogLevels

func (f *logLevelsFlag) Type() string {
	return "stringToString"
}

func (f *logLevelsFlag) Set(val string) error {
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")

	parsed := DefaultLogLevels()

	for val != "" {
		pair, rest, _ := strings.Cut(val, ",")
		val = rest
		k, v, ok := strings.Cut(pair, "=")

		if k == "" {
			return fmt.Errorf("component name cannot be empty: %q", pair)
		}
		if !ok {
			return fmt.Errorf("must be of format component=level: %q", pair)
		}

		switch k {
		case "containerd":
			parsed.Containerd = v
		case "etcd":
			parsed.Etcd = v
		case "konnectivity-server":
			parsed.Konnectivity = v
		case "kube-apiserver":
			parsed.KubeAPIServer = v
		case "kube-controller-manager":
			parsed.KubeControllerManager = v
		case "kube-scheduler":
			parsed.KubeScheduler = v
		case "kubelet":
			parsed.Kubelet = v
		default:
			return fmt.Errorf("unknown component name: %q", k)
		}
	}

	*f = parsed
	return nil
}

func (f *logLevelsFlag) String() string {
	var buf strings.Builder
	buf.WriteString("[containerd=")
	buf.WriteString(f.Containerd)
	buf.WriteString(",etcd=")
	buf.WriteString(f.Etcd)
	buf.WriteString(",konnectivity-server=")
	buf.WriteString(f.Konnectivity)
	buf.WriteString(",kube-apiserver=")
	buf.WriteString(f.KubeAPIServer)
	buf.WriteString(",kube-controller-manager=")
	buf.WriteString(f.KubeControllerManager)
	buf.WriteString(",kube-scheduler=")
	buf.WriteString(f.KubeScheduler)
	buf.WriteString(",kubelet=")
	buf.WriteString(f.Kubelet)
	buf.WriteString("]")
	return buf.String()
}

func GetPersistentFlagSet() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.String("data-dir", constant.DataDirDefault, "Data Directory for k0s. DO NOT CHANGE for an existing setup, things will break!")
	flagset.String("status-socket", "", "Full file path to the socket file. (default: <rundir>/status.sock)")
	return flagset
}

func GetKubeCtlFlagSet() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.String("data-dir", constant.DataDirDefault, "Data Directory for k0s. DO NOT CHANGE for an existing setup, things will break!")
	return flagset
}

func GetCriSocketFlag() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.StringVar(&workerOpts.CriSocket, "cri-socket", "", "container runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]")
	return flagset
}

func GetWorkerFlags() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}

	if workerOpts.LogLevels == (LogLevels{}) {
		// initialize zero value with defaults
		workerOpts.LogLevels = DefaultLogLevels()
	}

	flagset.String("cidr-range", "", "")
	flagset.VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
	})

	if workerOpts.Labels == nil {
		// cliflag.ConfigurationMap expects the map to be non-nil.
		workerOpts.Labels = make(map[string]string)
	}

	var defaultWorkerProfile string
	if runtime.GOOS == "windows" {
		defaultWorkerProfile = "default-windows"
	} else {
		defaultWorkerProfile = "default"
	}

	flagset.String("kubelet-root-dir", "", "Kubelet root directory for k0s")
	flagset.StringVar(&workerOpts.WorkerProfile, "profile", defaultWorkerProfile, "worker profile to use on the node")
	flagset.BoolVar(&workerOpts.CloudProvider, "enable-cloud-provider", false, "Whether or not to enable cloud provider support in kubelet")
	flagset.StringVar(&workerOpts.TokenFile, "token-file", "", "Path to the file containing join-token.")
	flagset.VarP((*logLevelsFlag)(&workerOpts.LogLevels), "logging", "l", "Logging Levels for the different components")
	flagset.Var((*cliflag.ConfigurationMap)(&workerOpts.Labels), "labels", "Node labels, list of key=value pairs")
	flagset.StringSliceVarP(&workerOpts.Taints, "taints", "", []string{}, "Node taints, list of key=value:effect strings")
	flagset.StringVar(&workerOpts.KubeletExtraArgs, "kubelet-extra-args", "", "extra args for kubelet")
	flagset.StringVar(&workerOpts.IPTablesMode, "iptables-mode", "", "iptables mode (valid values: nft, legacy, auto). default: auto")
	flagset.AddFlagSet(GetCriSocketFlag())

	return flagset
}

var availableComponents = []string{
	constant.ApplierManagerComponentName,
	constant.AutopilotComponentName,
	constant.ControlAPIComponentName,
	constant.CoreDNSComponentname,
	constant.CsrApproverComponentName,
	constant.APIEndpointReconcilerComponentName,
	constant.HelmComponentName,
	constant.KonnectivityServerComponentName,
	constant.KubeControllerManagerComponentName,
	constant.KubeProxyComponentName,
	constant.KubeSchedulerComponentName,
	constant.MetricsServerComponentName,
	constant.NetworkProviderComponentName,
	constant.NodeRoleComponentName,
	constant.SystemRBACComponentName,
	constant.UpdateProberComponentName,
	constant.WindowsNodeComponentName,
	constant.WorkerConfigComponentName,
}

func GetControllerFlags(controllerOpts *ControllerOptions) *pflag.FlagSet {
	flagset := &pflag.FlagSet{}

	flagset.BoolVar(&controllerOpts.enableWorker, "enable-worker", false, "enable worker (default false)")
	flagset.StringSliceVar(&controllerOpts.DisableComponents, "disable-components", []string{}, "disable components (valid items: "+strings.Join(availableComponents, ",")+")")
	flagset.BoolVar(&controllerOpts.singleNode, "single", false, "enable single node (implies --enable-worker, default false)")
	flagset.BoolVar(&controllerOpts.NoTaints, "no-taints", false, "disable default taints for controller node")
	flagset.BoolVar(&controllerOpts.EnableK0sCloudProvider, "enable-k0s-cloud-provider", false, "enables the k0s-cloud-provider (default false)")
	flagset.DurationVar(&controllerOpts.K0sCloudProviderUpdateFrequency, "k0s-cloud-provider-update-frequency", 2*time.Minute, "the frequency of k0s-cloud-provider node updates")
	flagset.IntVar(&controllerOpts.K0sCloudProviderPort, "k0s-cloud-provider-port", k0scloudprovider.DefaultBindPort, "the port that k0s-cloud-provider binds on")
	flagset.BoolVar(&controllerOpts.EnableDynamicConfig, "enable-dynamic-config", false, "enable cluster-wide dynamic config based on custom resource")
	flagset.BoolVar(&controllerOpts.EnableMetricsScraper, "enable-metrics-scraper", false, "enable scraping metrics from the controller components (kube-scheduler, kube-controller-manager)")
	flagset.StringVar(&controllerOpts.KubeControllerManagerExtraArgs, "kube-controller-manager-extra-args", "", "extra args for kube-controller-manager")
	flagset.BoolVar(&controllerOpts.InitOnly, "init-only", false, "only initialize controller and exit")
	flagset.Var(&controllerOpts.FeatureGates, "feature-gates", "feature gates to enable (comma separated list of key=value pairs)")
	return flagset
}

// The config flag used to be a persistent, joint flag to all commands
// now only a few commands use it. This function helps to share the flag with multiple commands without needing to define
// it in multiple places
func FileInputFlag() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	descString := fmt.Sprintf("config file, use '-' to read the config from stdin (default %q)", constant.K0sConfigPathDefault)
	flagset.StringVarP(&CfgFile, "config", "c", "", descString)

	return flagset
}

func GetCmdOpts(cobraCmd command) (*CLIOptions, error) {
	k0sVars, err := NewCfgVars(cobraCmd)
	if err != nil {
		return nil, err
	}

	// if a runtime config can be loaded, use it to override the k0sVars
	if rtc, err := LoadRuntimeConfig(k0sVars.RuntimeConfigPath); err == nil {
		k0sVars = rtc.K0sVars
	}

	return &CLIOptions{
		WorkerOptions: workerOpts,

		CfgFile: CfgFile,
		K0sVars: k0sVars,
	}, nil
}
