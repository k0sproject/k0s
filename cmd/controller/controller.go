//go:build unix

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/k0sproject/k0s/cmd/internal"
	workercmd "github.com/k0sproject/k0s/cmd/worker"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/internal/supervised"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/updates"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/controller/clusterconfig"
	"github.com/k0sproject/k0s/pkg/component/controller/cplb"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/controller/workerconfig"
	"github.com/k0sproject/k0s/pkg/component/iptables"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"
	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/k0sproject/k0s/pkg/telemetry"
	"github.com/k0sproject/k0s/pkg/token"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/ptr"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command config.CLIOptions

func NewControllerCmd() *cobra.Command {
	var (
		debugFlags            internal.DebugFlags
		controllerFlags       config.ControllerOptions
		ignorePreFlightChecks bool
	)

	cmd := &cobra.Command{
		Use:     "controller [join-token]",
		Short:   "Run controller",
		Aliases: []string{"server"},
		Example: `	Command to associate master nodes:
	CLI argument:
	$ k0s controller [join-token]

	or CLI flag:
	$ k0s controller --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag`,
		Args:             cobra.MaximumNArgs(1),
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			c := (*command)(opts)

			if len(args) > 0 {
				c.TokenArg = args[0]
			}
			if c.TokenArg != "" && c.TokenFile != "" {
				return errors.New("you can only pass one token argument either as a CLI argument 'k0s controller [join-token]' or as a flag 'k0s controller --token-file [path]'")
			}
			if err := controllerFlags.Normalize(); err != nil {
				return err
			}

			if err := (&sysinfo.K0sSysinfoSpec{
				ControllerRoleEnabled: true,
				WorkerRoleEnabled:     controllerFlags.Mode().WorkloadsEnabled(),
				DataDir:               c.K0sVars.DataDir,
			}).RunPreFlightChecks(ignorePreFlightChecks); !ignorePreFlightChecks && err != nil {
				return err
			}

			ctx := cmd.Context()
			if err := c.start(ctx, &controllerFlags, debugFlags.IsDebug()); err != nil {
				return err
			}

			// Components may have stopped themselves (for example, after an
			// etcd leave request), but the process lifetime is still governed
			// by external signals/init systems.
			select {
			case <-ctx.Done():
			default:
				logrus.Info("Awaiting process shutdown")
				<-ctx.Done()
				logrus.Info("Shutting down process: ", context.Cause(ctx))
			}

			return nil
		},
	}

	debugFlags.LongRunning().AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetControllerFlags(&controllerFlags))
	flags.AddFlagSet(config.GetWorkerFlags())
	flags.AddFlagSet(config.FileInputFlag())
	flags.BoolVar(&ignorePreFlightChecks, "ignore-pre-flight-checks", false, "continue even if pre-flight checks fail")

	return cmd
}

func (c *command) start(ctx context.Context, flags *config.ControllerOptions, debug bool) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	perfTimer := performance.NewTimer("controller-start").Buffer().Start()

	nodeConfig, err := c.K0sVars.NodeConfig()
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	if errs := nodeConfig.Validate(); len(errs) > 0 {
		return fmt.Errorf("invalid node config: %w", errors.Join(errs...))
	}

	nodeComponents := manager.New(prober.DefaultProber)
	clusterComponents := manager.New(prober.DefaultProber)

	// create directories early with the proper permissions
	if err := dir.Init(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
		return err
	}
	if err := dir.Init(c.K0sVars.RunDir, constant.RunDirMode); err != nil {
		return err
	}
	if err := dir.Init(c.K0sVars.BinDir, constant.BinDirMode); err != nil {
		return err
	}
	if err := dir.Init(c.K0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}

	rtc, err := config.NewRuntimeConfig(c.K0sVars, nodeConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime config: %w", err)
	}
	defer func() {
		if err := rtc.Spec.Cleanup(); err != nil {
			logrus.WithError(err).Warn("Failed to cleanup runtime config")
		}
	}()

	// common factory to get the admin kube client that's needed in many components
	adminClientFactory := &kubernetes.ClientFactory{LoadRESTConfig: func() (*rest.Config, error) {
		config, err := kubernetes.ClientConfig(kubernetes.KubeconfigFromFile(c.K0sVars.AdminKubeConfigPath))
		if err != nil {
			return nil, err
		}

		// We're always running the client on the same host as the API, no need to compress
		config.DisableCompression = true
		// To mitigate stack applier bursts in startup
		config.QPS = 40.0
		config.Burst = 400.0

		return config, nil
	}}

	certificateManager := certificate.Manager{K0sVars: c.K0sVars}

	var joinClient *token.JoinClient

	if (c.TokenArg != "" || c.TokenFile != "") && c.needToJoin(nodeConfig) {
		var tokenData string
		if c.TokenArg != "" {
			tokenData = c.TokenArg
		} else {
			data, err := os.ReadFile(c.TokenFile)
			if err != nil {
				return fmt.Errorf("read token file %q: %w", c.TokenFile, err)
			}
			tokenData = string(data)
		}
		joinClient, err = joinController(ctx, tokenData, c.K0sVars.CertRootDir)
		if err != nil {
			return fmt.Errorf("failed to join controller: %w", err)
		}
	}

	logrus.Infof("using api address: %s", nodeConfig.Spec.API.Address)
	logrus.Infof("using listen port: %d", nodeConfig.Spec.API.Port)
	logrus.Infof("using sans: %s", nodeConfig.Spec.API.SANs)

	dnsAddress, err := nodeConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)

	var storageBackend manager.Component
	storageType := nodeConfig.Spec.Storage.Type

	switch storageType {
	case v1beta1.KineStorageType:
		storageBackend = &controller.Kine{
			Config:  nodeConfig.Spec.Storage.Kine,
			K0sVars: c.K0sVars,
		}
	case v1beta1.EtcdStorageType:
		storageBackend = &controller.Etcd{
			CertManager: certificateManager,
			Config:      nodeConfig.Spec.Storage.Etcd,
			JoinClient:  joinClient,
			K0sVars:     c.K0sVars,
			LogLevel:    c.LogLevels.Etcd,
		}
	default:
		return fmt.Errorf("invalid storage type: %s", nodeConfig.Spec.Storage.Type)
	}
	logrus.Infof("using storage backend %s", nodeConfig.Spec.Storage.Type)
	nodeComponents.Add(ctx, storageBackend)

	controllerMode := flags.Mode()
	// Will the cluster support multiple controllers, or just a single one?
	singleController := controllerMode == config.SingleNodeMode || !nodeConfig.Spec.Storage.IsJoinable()

	// Assume a single active controller during startup
	numActiveControllers := value.NewLatest[uint](1)

	nodeComponents.Add(ctx, &iptables.Component{
		IPTablesMode: c.IPTablesMode,
		BinDir:       c.K0sVars.BinDir,
	})

	enableK0sEndpointReconciler := nodeConfig.Spec.API.ExternalAddress != "" &&
		!slices.Contains(flags.DisableComponents, constant.APIEndpointReconcilerComponentName)

	if cplbCfg := nodeConfig.Spec.Network.ControlPlaneLoadBalancing; cplbCfg != nil && cplbCfg.Enabled {
		if controllerMode == config.SingleNodeMode {
			return errors.New("control plane load balancing cannot be used in a single-node cluster")
		}

		if enableK0sEndpointReconciler {
			enableK0sEndpointReconciler = false
			logrus.Info("Disabling k0s endpoint reconciler in favor of control plane load balancing")
		}

		nodeComponents.Add(ctx, &cplb.Keepalived{
			K0sVars:         c.K0sVars,
			Config:          cplbCfg.Keepalived,
			DetailedLogging: debug,
			LogConfig:       debug,
			KubeConfigPath:  c.K0sVars.AdminKubeConfigPath,
			APIPort:         nodeConfig.Spec.API.Port,
		})
	}

	enableKonnectivity := controllerMode != config.SingleNodeMode && !slices.Contains(flags.DisableComponents, constant.KonnectivityServerComponentName)

	if enableKonnectivity {
		nodeComponents.Add(ctx, &controller.Konnectivity{
			Spec:         nodeConfig.Spec.Konnectivity,
			K0sVars:      c.K0sVars,
			LogLevel:     c.LogLevels.Konnectivity,
			EventEmitter: prober.NewEventEmitter(),
			ServerCount:  numActiveControllers.Peek,
		})
	}

	nodeComponents.Add(ctx, &controller.APIServer{
		ClusterConfig:      nodeConfig,
		K0sVars:            c.K0sVars,
		LogLevel:           c.LogLevels.KubeAPIServer,
		Storage:            storageBackend,
		EnableKonnectivity: enableKonnectivity,

		// If k0s reconciles the kubernetes endpoint, the API server shouldn't do it.
		DisableEndpointReconciler: enableK0sEndpointReconciler,
	})

	nodeName, kubeletExtraArgs, err := workercmd.GetNodeName(&c.WorkerOptions)
	if err != nil {
		return fmt.Errorf("failed to determine node name: %w", err)
	}

	if !singleController {
		nodeComponents.Add(ctx, &controller.K0sControllersLeaseCounter{
			NodeName:              nodeName,
			InvocationID:          c.K0sVars.InvocationID,
			ClusterConfig:         nodeConfig,
			KubeClientFactory:     adminClientFactory,
			UpdateControllerCount: numActiveControllers.Set,
		})
	}

	var leaderElector interface {
		leaderelector.Interface
		manager.Component
	}

	// One leader elector per controller
	if singleController {
		leaderElector = leaderelector.Off()
	} else {
		// The name used to be hardcoded in the component itself
		// At some point we need to rename this.
		leaderElector = leaderelector.NewLeasePool(c.K0sVars.InvocationID, adminClientFactory, "k0s-endpoint-reconciler")
	}
	nodeComponents.Add(ctx, leaderElector)

	if !slices.Contains(flags.DisableComponents, constant.ApplierManagerComponentName) {
		nodeComponents.Add(ctx, &applier.Manager{
			K0sVars:           c.K0sVars,
			KubeClientFactory: adminClientFactory,
			IgnoredStacks: []string{
				controller.AutopilotStackName,
				controller.ClusterConfigStackName,
				controller.EtcdMemberStackName,
				controller.SystemRBACStackName,
				controller.WindowsStackName,
			},
			LeaderElector: leaderElector,
		})
	}

	if !slices.Contains(flags.DisableComponents, constant.ControlAPIComponentName) && nodeConfig.Spec.Storage.IsJoinable() {
		nodeComponents.Add(ctx, &controller.K0SControlAPI{RuntimeConfig: rtc})
	}

	if !slices.Contains(flags.DisableComponents, constant.CsrApproverComponentName) {
		nodeComponents.Add(ctx, controller.NewCSRApprover(nodeConfig,
			leaderElector,
			adminClientFactory))
	}

	if flags.EnableK0sCloudProvider {
		nodeComponents.Add(
			ctx,
			controller.NewK0sCloudProvider(
				c.K0sVars.AdminKubeConfigPath,
				flags.K0sCloudProviderUpdateFrequency,
				flags.K0sCloudProviderPort,
			),
		)
	}
	nodeComponents.Add(ctx, &status.Status{
		Prober: prober.DefaultProber,
		StatusInformation: status.K0sStatus{
			Pid:           os.Getpid(),
			Role:          "controller",
			Args:          os.Args,
			Version:       build.Version,
			Workloads:     controllerMode.WorkloadsEnabled(),
			SingleNode:    controllerMode == config.SingleNodeMode,
			K0sVars:       c.K0sVars,
			ClusterConfig: nodeConfig,
		},
		Socket:      c.K0sVars.StatusSocketPath,
		CertManager: worker.NewCertificateManager(c.K0sVars.KubeletAuthConfigPath),
	})

	perfTimer.Checkpoint("starting-certificates-init")
	certs := &Certificates{
		ClusterSpec: nodeConfig.Spec,
		CertManager: certificateManager,
		K0sVars:     c.K0sVars,
	}
	if err := certs.Init(ctx); err != nil {
		return err
	}

	perfTimer.Checkpoint("starting-node-component-init")
	// init Node components
	if err := nodeComponents.Init(ctx); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-node-component-init")

	perfTimer.Checkpoint("starting-node-components")

	if flags.InitOnly {
		return nil
	}

	// Start components
	err = nodeComponents.Start(ctx)
	perfTimer.Checkpoint("finished-starting-node-components")
	if err != nil {
		return fmt.Errorf("failed to start controller node components: %w", err)
	}
	defer func() {
		// Stop components
		if err := nodeComponents.Stop(); err != nil {
			logrus.WithError(err).Error("Failed to stop node components")
		} else {
			logrus.Info("All node components stopped")
		}
	}()

	var configSource clusterconfig.ConfigSource
	// For backwards compatibility, use file as config source by default
	if flags.EnableDynamicConfig {
		clusterComponents.Add(ctx, controller.NewClusterConfigInitializer(
			adminClientFactory,
			func() leaderelection.Status { status, _ := leaderElector.CurrentStatus(); return status },
			nodeConfig,
		))

		configSource, err = clusterconfig.NewAPIConfigSource(adminClientFactory)
		if err != nil {
			return err
		}
	} else {
		configSource = clusterconfig.NewStaticSource(nodeConfig)
	}

	clusterComponents.Add(ctx, controller.NewClusterConfigReconciler(
		clusterComponents,
		adminClientFactory,
		configSource,
	))

	if !slices.Contains(flags.DisableComponents, constant.HelmComponentName) {
		clusterComponents.Add(ctx, controller.NewCRD(c.K0sVars.ManifestsDir, controller.HelmExtensionStackName))
		clusterComponents.Add(ctx, controller.NewExtensionsController(
			c.K0sVars,
			adminClientFactory,
			leaderElector,
		))
	}

	if enableK0sEndpointReconciler {
		clusterComponents.Add(ctx, controller.NewEndpointReconciler(
			nodeConfig,
			func() leaderelection.Status { status, _ := leaderElector.CurrentStatus(); return status },
			adminClientFactory,
			net.DefaultResolver,
			nodeConfig.PrimaryAddressFamily(),
		))
	}

	hasWindowsNodes := func() (*bool, <-chan struct{}) { return ptr.To(false), nil }
	if !slices.Contains(flags.DisableComponents, constant.WindowsNodeComponentName) {
		var windowsNodeCount value.Latest[*uint]
		windowsStack, err := controller.NewWindowsStackComponent(adminClientFactory, windowsNodeCount.Set)
		if err != nil {
			return fmt.Errorf("failed to create Windows stack component: %w", err)
		}
		clusterComponents.Add(ctx, windowsStack)
		hasWindowsNodes = func() (*bool, <-chan struct{}) {
			count, changed := windowsNodeCount.Peek()
			if count != nil {
				return ptr.To(*count > 0), changed
			}
			return nil, changed
		}
	}

	if !slices.Contains(flags.DisableComponents, constant.KubeProxyComponentName) {
		clusterComponents.Add(ctx, controller.NewKubeProxy(c.K0sVars, nodeConfig, hasWindowsNodes))
	}

	if !slices.Contains(flags.DisableComponents, constant.CoreDNSComponentname) {
		coreDNS, err := controller.NewCoreDNS(c.K0sVars, adminClientFactory, nodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create CoreDNS reconciler: %w", err)
		}
		clusterComponents.Add(ctx, coreDNS)
	}

	if !slices.Contains(flags.DisableComponents, constant.NetworkProviderComponentName) {
		logrus.Infof("Creating network reconcilers")
		calico, err := controller.NewCalico(nodeConfig, c.K0sVars.ManifestsDir, hasWindowsNodes)
		if err != nil {
			return fmt.Errorf("failed to create Calico component: %w", err)
		}
		clusterComponents.Add(ctx, calico)
		clusterComponents.Add(ctx, controller.NewKubeRouter(c.K0sVars))
	}

	if !slices.Contains(flags.DisableComponents, constant.MetricsServerComponentName) {
		if !enableKonnectivity && !flags.Mode().WorkloadsEnabled() {
			logrus.Warn("In order to run metrics-server without konnectivity, this controller must be able to connect to the cluster network")
		}
		clusterComponents.Add(ctx, controller.NewMetricServer(c.K0sVars, adminClientFactory))
	}

	if flags.EnableMetricsScraper {
		metrics, err := controller.NewMetrics(c.K0sVars, adminClientFactory, nodeConfig.Spec.Storage.Type)
		if err != nil {
			return fmt.Errorf("failed to create metrics reconciler: %w", err)
		}
		clusterComponents.Add(ctx, metrics)
	}

	disableAutopilot := slices.Contains(flags.DisableComponents, constant.AutopilotComponentName)

	if !slices.Contains(flags.DisableComponents, constant.WorkerConfigComponentName) {
		// Create new dedicated leasepool for worker config reconciler
		leaseName := fmt.Sprintf("k0s-%s-%s", constant.WorkerConfigComponentName, constant.KubernetesMajorMinorVersion)
		workerConfigLeasePool := leaderelector.NewLeasePool(c.K0sVars.InvocationID, adminClientFactory, leaseName)
		clusterComponents.Add(ctx, workerConfigLeasePool)

		reconciler, err := workerconfig.NewReconciler(c.K0sVars, nodeConfig.Spec, adminClientFactory, workerConfigLeasePool, enableKonnectivity, disableAutopilot)
		if err != nil {
			return err
		}
		clusterComponents.Add(ctx, reconciler)
	}

	if !slices.Contains(flags.DisableComponents, constant.SystemRBACComponentName) {
		clusterComponents.Add(ctx, &controller.SystemRBAC{
			Clients:          adminClientFactory,
			ExcludeAutopilot: disableAutopilot,
		})
	}

	if !slices.Contains(flags.DisableComponents, constant.NodeRoleComponentName) {
		clusterComponents.Add(ctx, controller.NewNodeRole(c.K0sVars, adminClientFactory))
	}

	if enableKonnectivity {
		clusterComponents.Add(ctx, &controller.KonnectivityAgent{
			ManifestsDir:           c.K0sVars.ManifestsDir,
			KonnectivityServerHost: cmp.Or(nodeConfig.Spec.API.ExternalHost(), nodeConfig.Spec.API.Address),
			EventEmitter:           prober.NewEventEmitter(),
			ServerCount:            numActiveControllers.Peek,
		})
	}

	if !slices.Contains(flags.DisableComponents, constant.KubeSchedulerComponentName) {
		clusterComponents.Add(ctx, &controller.Scheduler{
			LogLevel:              c.LogLevels.KubeScheduler,
			K0sVars:               c.K0sVars,
			DisableLeaderElection: singleController,
		})
	}

	if !slices.Contains(flags.DisableComponents, constant.KubeControllerManagerComponentName) {
		clusterComponents.Add(ctx, &controller.Manager{
			LogLevel:              c.LogLevels.KubeControllerManager,
			K0sVars:               c.K0sVars,
			DisableLeaderElection: singleController,
			ServiceClusterIPRange: nodeConfig.Spec.Network.BuildServiceCIDR(nodeConfig.PrimaryAddressFamily()),
			ExtraArgs:             flags.KubeControllerManagerExtraArgs,
		})
	}

	if nodeConfig.Spec.Storage.Type == v1beta1.EtcdStorageType && !nodeConfig.Spec.Storage.Etcd.IsExternalClusterUsed() {
		etcdReconciler, err := controller.NewEtcdMemberReconciler(
			adminClientFactory,
			c.K0sVars,
			nodeConfig.Spec.Storage.Etcd,
			leaderElector,
			func() uint {
				num, _ := numActiveControllers.Peek()
				return num
			},
			cancel,
		)
		if err != nil {
			return err
		}
		clusterComponents.Add(ctx, controller.NewCRD(c.K0sVars.ManifestsDir, "etcd", controller.WithStackName("etcd-member")))
		clusterComponents.Add(ctx, etcdReconciler)
	}

	if telemetry.IsEnabled() {
		clusterComponents.Add(ctx, &telemetry.Component{
			K0sVars:           c.K0sVars,
			StorageType:       storageType,
			KubeClientFactory: adminClientFactory,
		})
	} else {
		logrus.Info("Telemetry is disabled")
	}

	if !disableAutopilot {
		client, err := adminClientFactory.GetClient()
		if err != nil {
			return err
		}
		apiAddress, err := netip.ParseAddr(nodeConfig.Spec.API.Address)
		if err != nil {
			return fmt.Errorf("failed to parse API address: %w", err)
		}

		clusterComponents.Add(ctx, controller.NewCRDStack(adminClientFactory, leaderElector, controller.AutopilotStackName))
		clusterComponents.Add(ctx, &controller.Autopilot{
			APIAddress:           apiAddress,
			K0sVars:              c.K0sVars,
			KubeletExtraArgs:     c.KubeletExtraArgs,
			KubeAPIPort:          nodeConfig.Spec.API.Port,
			AdminClientFactory:   adminClientFactory,
			ClusterInfoCollector: updates.NewClusterInfoCollector(nodeConfig, client),
			Workloads:            controllerMode.WorkloadsEnabled(),
		})
	}

	if !slices.Contains(flags.DisableComponents, constant.UpdateProberComponentName) {
		client, err := adminClientFactory.GetClient()
		if err != nil {
			return err
		}
		clusterComponents.Add(ctx, controller.NewUpdateProber(
			adminClientFactory,
			leaderElector,
			updates.NewClusterInfoCollector(nodeConfig, client),
		))
	}

	// Add the config source as the last component, so that the reconciliation
	// starts after all other components have been started.
	clusterComponents.Add(ctx, configSource)

	perfTimer.Checkpoint("starting-cluster-components-init")
	// init Cluster components
	if err := clusterComponents.Init(ctx); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished cluster-component-init")

	err = clusterComponents.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start cluster components: %w", err)
	}
	perfTimer.Checkpoint("finished-starting-cluster-components")
	defer func() {
		// Stop Cluster components
		if err := clusterComponents.Stop(); err != nil {
			logrus.WithError(err).Error("Failed to stop cluster components")
		} else {
			logrus.Info("All cluster components stopped")
		}
	}()

	perfTimer.Output()

	if controllerMode.WorkloadsEnabled() {
		return c.startWorker(ctx, nodeName, kubeletExtraArgs, flags)
	}

	if supervised := supervised.Get(ctx); supervised != nil {
		supervised.MarkReady()
	}

	// Wait for k0s process termination
	logrus.Info("Controller has started")
	<-ctx.Done()
	logrus.Info("Shutting down k0s: ", context.Cause(ctx))

	return nil
}

func (c *command) startWorker(ctx context.Context, nodeName apitypes.NodeName, kubeletExtraArgs stringmap.StringMap, opts *config.ControllerOptions) error {
	// Cast and make a copy of the controller command so it can use the same
	// opts to start the worker. Needs to be a copy so the original token and
	// possibly other args won't get messed up.
	wc := workercmd.Command(*(*config.CLIOptions)(c))
	wc.Labels[constant.K0SNodeRoleLabel] = "control-plane"
	if opts.Mode() == config.ControllerPlusWorkerMode && !opts.NoTaints {
		wc.Taints = append(wc.Taints, constants.ControlPlaneTaint.ToString())
	}
	return wc.Start(ctx, nodeName, kubeletExtraArgs, kubernetes.KubeconfigFromFile(c.K0sVars.AdminKubeConfigPath), (*embeddingController)(opts))
}

type embeddingController config.ControllerOptions

// IsSingleNode implements [workercmd.EmbeddingController].
func (c *embeddingController) IsSingleNode() bool {
	return (*config.ControllerOptions)(c).Mode() == config.SingleNodeMode
}

// If we've got an etcd data directory in place for embedded etcd, or a ca for
// external or other storage types, we assume the node has already joined
// previously.
func (c *command) needToJoin(nodeConfig *v1beta1.ClusterConfig) bool {
	if nodeConfig.Spec.Storage.Type == v1beta1.EtcdStorageType && !nodeConfig.Spec.Storage.Etcd.IsExternalClusterUsed() {
		// Use the main etcd data directory as the source of truth to determine if this node has already joined
		// See https://etcd.io/docs/v3.5/learning/persistent-storage-files/#bbolt-btree-membersnapdb
		return !file.Exists(filepath.Join(c.K0sVars.EtcdDataDir, "member", "snap", "db"))
	}
	if file.Exists(filepath.Join(c.K0sVars.CertRootDir, "ca.key")) &&
		file.Exists(filepath.Join(c.K0sVars.CertRootDir, "ca.crt")) {
		return false
	}
	return true
}

func writeCerts(caData v1beta1.CaResponse, certRootDir string) error {
	type fileData struct {
		path string
		data []byte
		mode fs.FileMode
	}
	for _, f := range []fileData{
		{path: filepath.Join(certRootDir, "ca.key"), data: caData.Key, mode: constant.CertSecureMode},
		{path: filepath.Join(certRootDir, "ca.crt"), data: caData.Cert, mode: constant.CertMode},
		{path: filepath.Join(certRootDir, "sa.key"), data: caData.SAKey, mode: constant.CertSecureMode},
		{path: filepath.Join(certRootDir, "sa.pub"), data: caData.SAPub, mode: constant.CertMode},
	} {
		err := file.WriteContentAtomically(f.path, f.data, f.mode)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", f.path, err)
		}
	}
	return nil
}

func joinController(ctx context.Context, tokenArg string, certRootDir string) (*token.JoinClient, error) {
	joinClient, err := token.JoinClientFromToken(tokenArg)
	if err != nil {
		return nil, fmt.Errorf("failed to create join client: %w", err)
	}

	logrus.Info("Joining existing cluster via ", joinClient.Address())

	var caData v1beta1.CaResponse
	retryErr := retry.Do(
		func() error {
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			caData, err = joinClient.GetCA(ctx)
			return err
		},
		retry.Context(ctx),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(attempt uint, err error) {
			logrus.WithError(err).Debug("Failed to join in attempt #", attempt+1, ", retrying after backoff")
		}),
	)
	if retryErr != nil {
		if err != nil {
			retryErr = err
		}
		return nil, fmt.Errorf("failed to join existing cluster via %s: %w", joinClient.Address(), retryErr)
	}

	logrus.Info("Got valid CA response, storing certificates")
	return joinClient, writeCerts(caData, certRootDir)
}
