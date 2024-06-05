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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	workercmd "github.com/k0sproject/k0s/cmd/worker"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0slog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	apclient "github.com/k0sproject/k0s/pkg/autopilot/client"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/controller/clusterconfig"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/controller/workerconfig"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/k0sproject/k0s/pkg/telemetry"
	"github.com/k0sproject/k0s/pkg/token"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command config.CLIOptions

func NewControllerCmd() *cobra.Command {
	var ignorePreFlightChecks bool

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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			logrus.SetOutput(cmd.OutOrStdout())
			k0slog.SetInfoLevel()
			return config.CallParentPersistentPreRun(cmd, args)
		},
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
				return fmt.Errorf("you can only pass one token argument either as a CLI argument 'k0s controller [join-token]' or as a flag 'k0s controller --token-file [path]'")
			}
			if err := c.ControllerOptions.Normalize(); err != nil {
				return err
			}

			if err := (&sysinfo.K0sSysinfoSpec{
				ControllerRoleEnabled: true,
				WorkerRoleEnabled:     c.SingleNode || c.EnableWorker,
				DataDir:               c.K0sVars.DataDir,
			}).RunPreFlightChecks(ignorePreFlightChecks); !ignorePreFlightChecks && err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return c.start(ctx)
		},
	}

	// append flags
	cmd.Flags().BoolVar(&ignorePreFlightChecks, "ignore-pre-flight-checks", false, "continue even if pre-flight checks fail")
	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.PersistentFlags().AddFlagSet(config.GetControllerFlags())
	cmd.PersistentFlags().AddFlagSet(config.GetWorkerFlags())
	return cmd
}

func (c *command) start(ctx context.Context) error {
	perfTimer := performance.NewTimer("controller-start").Buffer().Start()

	nodeConfig, err := c.K0sVars.NodeConfig()
	if err != nil {
		return fmt.Errorf("failed to load node config: %w", err)
	}

	if errs := nodeConfig.Validate(); len(errs) > 0 {
		return fmt.Errorf("invalid node config: %w", errors.Join(errs...))
	}

	// Add the node config to the context so it can be used by components deep in the "stack"
	ctx = context.WithValue(ctx, k0scontext.ContextNodeConfigKey, nodeConfig)

	nodeComponents := manager.New(prober.DefaultProber)
	clusterComponents := manager.New(prober.DefaultProber)

	// create directories early with the proper permissions
	if err := dir.Init(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
		return err
	}
	if err := dir.Init(c.K0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}
	// let's make sure run-dir exists
	if err := dir.Init(c.K0sVars.RunDir, constant.RunDirMode); err != nil {
		return fmt.Errorf("failed to initialize dir: %w", err)
	}

	rtc, err := config.NewRuntimeConfig(c.K0sVars)
	if err != nil {
		return fmt.Errorf("failed to initialize runtime config: %w", err)
	}
	defer func() {
		if err := rtc.Cleanup(); err != nil {
			logrus.WithError(err).Warn("Failed to cleanup runtime config")
		}
	}()

	// common factory to get the admin kube client that's needed in many components
	adminClientFactory := kubernetes.NewAdminClientFactory(c.K0sVars.AdminKubeConfigPath)

	certificateManager := certificate.Manager{K0sVars: c.K0sVars}

	var joinClient *token.JoinClient

	if (c.TokenArg != "" || c.TokenFile != "") && c.needToJoin() {
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

	switch nodeConfig.Spec.Storage.Type {
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

	controllerLeaseCounter := &controller.K0sControllersLeaseCounter{
		InvocationID:      c.K0sVars.InvocationID,
		ClusterConfig:     nodeConfig,
		KubeClientFactory: adminClientFactory,
	}

	if !c.SingleNode {
		nodeComponents.Add(ctx, controllerLeaseCounter)
	}

	if cplb := nodeConfig.Spec.Network.ControlPlaneLoadBalancing; cplb != nil && cplb.Enabled {
		if c.SingleNode {
			return errors.New("control plane load balancing cannot be used in a single-node cluster")
		}

		nodeComponents.Add(ctx, &controller.Keepalived{
			K0sVars:         c.K0sVars,
			Config:          cplb.Keepalived,
			DetailedLogging: c.Debug,
			LogConfig:       c.Debug,
			KubeConfigPath:  c.K0sVars.AdminKubeConfigPath,
			APIPort:         nodeConfig.Spec.API.Port,
		})
	}

	enableKonnectivity := !c.SingleNode && !slices.Contains(c.DisableComponents, constant.KonnectivityServerComponentName)
	disableEndpointReconciler := !slices.Contains(c.DisableComponents, constant.APIEndpointReconcilerComponentName) &&
		nodeConfig.Spec.API.ExternalAddress != ""

	if enableKonnectivity {
		nodeComponents.Add(ctx, &controller.Konnectivity{
			SingleNode:                 c.SingleNode,
			LogLevel:                   c.LogLevels.Konnectivity,
			K0sVars:                    c.K0sVars,
			KubeClientFactory:          adminClientFactory,
			NodeConfig:                 nodeConfig,
			EventEmitter:               prober.NewEventEmitter(),
			K0sControllersLeaseCounter: controllerLeaseCounter,
		})
	}

	nodeComponents.Add(ctx, &controller.APIServer{
		ClusterConfig:             nodeConfig,
		K0sVars:                   c.K0sVars,
		LogLevel:                  c.LogLevels.KubeAPIServer,
		Storage:                   storageBackend,
		EnableKonnectivity:        enableKonnectivity,
		DisableEndpointReconciler: disableEndpointReconciler,
	})

	var leaderElector interface {
		leaderelector.Interface
		manager.Component
	}

	// One leader elector per controller
	if !c.SingleNode {
		// The name used to be hardcoded in the component itself
		// At some point we need to rename this.
		leaderElector = leaderelector.NewLeasePool(c.K0sVars.InvocationID, adminClientFactory, "k0s-endpoint-reconciler")
	} else {
		leaderElector = &leaderelector.Dummy{Leader: true}
	}
	nodeComponents.Add(ctx, leaderElector)

	if !slices.Contains(c.DisableComponents, constant.ApplierManagerComponentName) {
		nodeComponents.Add(ctx, &applier.Manager{
			K0sVars:           c.K0sVars,
			KubeClientFactory: adminClientFactory,
			LeaderElector:     leaderElector,
		})
	}

	if !c.SingleNode && !slices.Contains(c.DisableComponents, constant.ControlAPIComponentName) {
		nodeComponents.Add(ctx, &controller.K0SControlAPI{
			ConfigPath: c.CfgFile,
			K0sVars:    c.K0sVars,
		})
	}

	if !slices.Contains(c.DisableComponents, constant.CsrApproverComponentName) {
		nodeComponents.Add(ctx, controller.NewCSRApprover(nodeConfig,
			leaderElector,
			adminClientFactory))
	}

	if c.EnableK0sCloudProvider {
		nodeComponents.Add(
			ctx,
			controller.NewK0sCloudProvider(
				c.K0sVars.AdminKubeConfigPath,
				c.K0sCloudProviderUpdateFrequency,
				c.K0sCloudProviderPort,
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
			Workloads:     c.SingleNode || c.EnableWorker,
			SingleNode:    c.SingleNode,
			K0sVars:       c.K0sVars,
			ClusterConfig: nodeConfig,
		},
		Socket:      c.K0sVars.StatusSocketPath,
		CertManager: worker.NewCertificateManager(ctx, c.K0sVars.KubeletAuthConfigPath),
	})

	if nodeConfig.Spec.Storage.Type == v1beta1.EtcdStorageType && !nodeConfig.Spec.Storage.Etcd.IsExternalClusterUsed() {
		etcdReconciler, err := controller.NewEtcdMemberReconciler(adminClientFactory, c.K0sVars, nodeConfig.Spec.Storage.Etcd, leaderElector)
		if err != nil {
			return err
		}
		etcdCRDSaver, err := controller.NewManifestsSaver("etcd-member", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize etcd-member manifests saver: %w", err)
		}
		clusterComponents.Add(ctx, controller.NewCRD(etcdCRDSaver, "etcd"))
		nodeComponents.Add(ctx, etcdReconciler)
	}

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
	if c.EnableDynamicConfig {
		configSource, err = clusterconfig.NewAPIConfigSource(adminClientFactory)
	} else {
		configSource, err = clusterconfig.NewStaticSource(nodeConfig)
	}
	if err != nil {
		return err
	}
	defer configSource.Stop()

	// The CRDs are only required if the config is stored in the cluster.
	if configSource.NeedToStoreInitialConfig() {
		apiConfigSaver, err := controller.NewManifestsSaver("api-config", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize api-config manifests saver: %w", err)
		}

		clusterComponents.Add(ctx, controller.NewCRD(apiConfigSaver, "v1beta1", controller.WithCRDAssetsDir("k0s")))
	}

	cfgReconciler, err := controller.NewClusterConfigReconciler(
		leaderElector,
		c.K0sVars,
		clusterComponents,
		adminClientFactory,
		configSource,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize cluster-config reconciler: %w", err)
	}
	clusterComponents.Add(ctx, cfgReconciler)

	if !slices.Contains(c.DisableComponents, constant.HelmComponentName) {
		helmSaver, err := controller.NewManifestsSaver("helm", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize helm manifests saver: %w", err)
		}
		clusterComponents.Add(ctx, controller.NewCRD(helmSaver, "helm"))
		clusterComponents.Add(ctx, controller.NewExtensionsController(
			helmSaver,
			c.K0sVars,
			adminClientFactory,
			leaderElector,
		))
	}

	if !slices.Contains(c.DisableComponents, constant.AutopilotComponentName) {
		logrus.Debug("starting manifest saver")
		manifestsSaver, err := controller.NewManifestsSaver("autopilot", c.K0sVars.DataDir)
		if err != nil {
			logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
			return err
		}
		clusterComponents.Add(ctx, controller.NewCRD(manifestsSaver, "autopilot"))
	}

	if !slices.Contains(c.DisableComponents, constant.APIEndpointReconcilerComponentName) && nodeConfig.Spec.API.ExternalAddress != "" {
		clusterComponents.Add(ctx, controller.NewEndpointReconciler(
			nodeConfig,
			leaderElector,
			adminClientFactory,
			net.DefaultResolver,
		))
	}

	if !slices.Contains(c.DisableComponents, constant.KubeProxyComponentName) {
		clusterComponents.Add(ctx, controller.NewKubeProxy(c.K0sVars, nodeConfig))
	}

	if !slices.Contains(c.DisableComponents, constant.CoreDNSComponentname) {
		coreDNS, err := controller.NewCoreDNS(c.K0sVars, adminClientFactory, nodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create CoreDNS reconciler: %w", err)
		}
		clusterComponents.Add(ctx, coreDNS)
	}

	if !slices.Contains(c.DisableComponents, constant.NetworkProviderComponentName) {
		logrus.Infof("Creating network reconcilers")

		calicoSaver, err := controller.NewManifestsSaver("calico", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create calico manifests saver: %w", err)
		}
		calicoInitSaver, err := controller.NewManifestsSaver("calico_init", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create calico_init manifests saver: %w", err)
		}
		windowsStackSaver, err := controller.NewManifestsSaver("windows", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create windows manifests saver: %w", err)
		}
		clusterComponents.Add(ctx, controller.NewCalico(c.K0sVars, calicoInitSaver, calicoSaver))
		if !slices.Contains(c.DisableComponents, constant.WindowsNodeComponentName) {
			clusterComponents.Add(ctx, controller.NewWindowsStackComponent(c.K0sVars, adminClientFactory, windowsStackSaver))
		}
		kubeRouterSaver, err := controller.NewManifestsSaver("kuberouter", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create kuberouter manifests saver: %w", err)
		}
		clusterComponents.Add(ctx, controller.NewKubeRouter(c.K0sVars, kubeRouterSaver))
	}

	if !slices.Contains(c.DisableComponents, constant.MetricsServerComponentName) {
		clusterComponents.Add(ctx, controller.NewMetricServer(c.K0sVars, adminClientFactory))
	}

	if c.EnableMetricsScraper {
		metricsSaver, err := controller.NewManifestsSaver("metrics", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create metrics manifests saver: %w", err)
		}
		metrics, err := controller.NewMetrics(c.K0sVars, metricsSaver, adminClientFactory, nodeConfig.Spec.Storage.Type)
		if err != nil {
			return fmt.Errorf("failed to create metrics reconciler: %w", err)
		}
		clusterComponents.Add(ctx, metrics)
	}

	if !slices.Contains(c.DisableComponents, constant.WorkerConfigComponentName) {
		// Create new dedicated leasepool for worker config reconciler
		leaseName := fmt.Sprintf("k0s-%s-%s", constant.WorkerConfigComponentName, constant.KubernetesMajorMinorVersion)
		workerConfigLeasePool := leaderelector.NewLeasePool(c.K0sVars.InvocationID, adminClientFactory, leaseName)
		clusterComponents.Add(ctx, workerConfigLeasePool)

		reconciler, err := workerconfig.NewReconciler(c.K0sVars, nodeConfig.Spec, adminClientFactory, workerConfigLeasePool, enableKonnectivity)
		if err != nil {
			return err
		}
		clusterComponents.Add(ctx, reconciler)
	}

	if !slices.Contains(c.DisableComponents, constant.SystemRbacComponentName) {
		clusterComponents.Add(ctx, controller.NewSystemRBAC(c.K0sVars.ManifestsDir))
	}

	if !slices.Contains(c.DisableComponents, constant.NodeRoleComponentName) {
		clusterComponents.Add(ctx, controller.NewNodeRole(c.K0sVars, adminClientFactory))
	}

	if enableKonnectivity {
		clusterComponents.Add(ctx, &controller.KonnectivityAgent{
			SingleNode:                 c.SingleNode,
			LogLevel:                   c.LogLevels.Konnectivity,
			K0sVars:                    c.K0sVars,
			KubeClientFactory:          adminClientFactory,
			NodeConfig:                 nodeConfig,
			EventEmitter:               prober.NewEventEmitter(),
			K0sControllersLeaseCounter: controllerLeaseCounter,
		})
	}

	if !slices.Contains(c.DisableComponents, constant.KubeSchedulerComponentName) {
		clusterComponents.Add(ctx, &controller.Scheduler{
			LogLevel:   c.LogLevels.KubeScheduler,
			K0sVars:    c.K0sVars,
			SingleNode: c.SingleNode,
		})
	}

	if !slices.Contains(c.DisableComponents, constant.KubeControllerManagerComponentName) {
		clusterComponents.Add(ctx, &controller.Manager{
			LogLevel:              c.LogLevels.KubeControllerManager,
			K0sVars:               c.K0sVars,
			SingleNode:            c.SingleNode,
			ServiceClusterIPRange: nodeConfig.Spec.Network.BuildServiceCIDR(nodeConfig.Spec.API.Address),
			ExtraArgs:             c.KubeControllerManagerExtraArgs,
		})
	}

	clusterComponents.Add(ctx, &telemetry.Component{
		Version:           build.Version,
		K0sVars:           c.K0sVars,
		KubeClientFactory: adminClientFactory,
	})

	clusterComponents.Add(ctx, &controller.Autopilot{
		K0sVars:            c.K0sVars,
		AdminClientFactory: adminClientFactory,
		EnableWorker:       c.EnableWorker,
	})

	apClientFactory, err := apclient.NewClientFactory(adminClientFactory.GetRESTConfig())
	if err != nil {
		return err
	}
	clusterComponents.Add(ctx, controller.NewUpdateProber(apClientFactory, leaderElector))

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

	// At this point all the components should be initialized and running, thus we can release the config for reconcilers
	go configSource.Release(ctx)

	if c.EnableWorker {
		perfTimer.Checkpoint("starting-worker")
		if err := c.startWorker(ctx, c.WorkerProfile, nodeConfig); err != nil {
			logrus.WithError(err).Error("Failed to start controller worker")
		} else {
			perfTimer.Checkpoint("started-worker")
		}
	}

	perfTimer.Output()

	// Wait for k0s process termination
	<-ctx.Done()
	logrus.Debug("Context done in main")
	logrus.Info("Shutting down k0s controller")

	perfTimer.Output()

	return nil
}

func (c *command) startWorker(ctx context.Context, profile string, nodeConfig *v1beta1.ClusterConfig) error {
	var bootstrapConfig string
	if !file.Exists(c.K0sVars.KubeletAuthConfigPath) {
		// wait for controller to start up
		err := retry.Do(func() error {
			if !file.Exists(c.K0sVars.AdminKubeConfigPath) {
				return fmt.Errorf("file does not exist: %s", c.K0sVars.AdminKubeConfigPath)
			}
			return nil
		}, retry.Context(ctx))
		if err != nil {
			return err
		}

		err = retry.Do(func() error {
			// five minutes here are coming from maximum theoretical duration of kubelet bootstrap process
			// we use retry.Do with 10 attempts, back-off delay and delay duration 500 ms which gives us
			// 225 seconds here
			tokenAge := time.Second * 225
			cfg, err := token.CreateKubeletBootstrapToken(ctx, nodeConfig.Spec.API, c.K0sVars, token.RoleWorker, tokenAge)
			if err != nil {
				return err
			}
			bootstrapConfig = cfg
			return nil
		}, retry.Context(ctx))
		if err != nil {
			return err
		}
	}
	// Cast and make a copy of the controller command so it can use the same
	// opts to start the worker. Needs to be a copy so the original token and
	// possibly other args won't get messed up.
	wc := workercmd.Command(*(*config.CLIOptions)(c))
	wc.TokenArg = bootstrapConfig
	wc.WorkerProfile = profile
	wc.Labels = append(wc.Labels, fmt.Sprintf("%s=control-plane", constant.K0SNodeRoleLabel))
	if !c.SingleNode && !c.NoTaints {
		wc.Taints = append(wc.Taints, fmt.Sprintf("%s/master=:NoSchedule", constant.NodeRoleLabelNamespace))
	}
	return wc.Start(ctx)
}

// If we've got CA in place we assume the node has already joined previously
func (c *command) needToJoin() bool {
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

	if joinClient.JoinTokenType() != "controller-bootstrap" {
		return nil, fmt.Errorf("wrong token type %s, expected type: controller-bootstrap", joinClient.JoinTokenType())
	}

	var caData v1beta1.CaResponse
	err = retry.Do(func() error {
		caData, err = joinClient.GetCA()
		if err != nil {
			return fmt.Errorf("failed to sync CA: %w", err)
		}
		return nil
	}, retry.Context(ctx))
	if err != nil {
		return nil, err
	}
	return joinClient, writeCerts(caData, certRootDir)
}
