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

package controller

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	workercmd "github.com/k0sproject/k0s/cmd/worker"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0slog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
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
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/k0sproject/k0s/pkg/telemetry"
	"github.com/k0sproject/k0s/pkg/token"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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
			c := command(config.GetCmdOpts())

			if len(args) > 0 {
				c.TokenArg = args[0]
			}
			if len(c.TokenArg) > 0 && len(c.TokenFile) > 0 {
				return fmt.Errorf("you can only pass one token argument either as a CLI argument 'k0s controller [join-token]' or as a flag 'k0s controller --token-file [path]'")
			}
			if err := c.ControllerOptions.Normalize(); err != nil {
				return err
			}
			if len(c.TokenFile) > 0 {
				bytes, err := os.ReadFile(c.TokenFile)
				if err != nil {
					return err
				}
				c.TokenArg = string(bytes)
			}
			c.Logging = stringmap.Merge(c.CmdLogLevels, c.DefaultLogLevels)
			cmd.SilenceUsage = true

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
	c.NodeComponents = manager.New(prober.DefaultProber)
	c.ClusterComponents = manager.New(prober.DefaultProber)

	perfTimer := performance.NewTimer("controller-start").Buffer().Start()

	// create directories early with the proper permissions
	if err := dir.Init(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
		return err
	}
	if err := dir.Init(c.K0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}
	// let's make sure run-dir exists
	if err := dir.Init(c.K0sVars.RunDir, constant.RunDirMode); err != nil {
		return fmt.Errorf("failed to initialize dir: %v", err)
	}

	// initialize runtime config
	loadingRules := config.ClientConfigLoadingRules{Nodeconfig: true}
	if err := loadingRules.InitRuntimeConfig(c.K0sVars); err != nil {
		return fmt.Errorf("failed to initialize k0s runtime config: %s", err.Error())
	}

	// from now on, we only refer to the runtime config
	c.CfgFile = loadingRules.RuntimeConfigPath

	certificateManager := certificate.Manager{K0sVars: c.K0sVars}

	var joinClient *token.JoinClient
	var err error

	if c.TokenArg != "" && c.needToJoin() {
		joinClient, err = joinController(ctx, c.TokenArg, c.K0sVars.CertRootDir)
		if err != nil {
			return fmt.Errorf("failed to join controller: %v", err)
		}
	}

	logrus.Infof("using api address: %s", c.NodeConfig.Spec.API.Address)
	logrus.Infof("using listen port: %d", c.NodeConfig.Spec.API.Port)
	logrus.Infof("using sans: %s", c.NodeConfig.Spec.API.SANs)
	dnsAddress, err := c.NodeConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)
	var storageBackend manager.Component

	switch c.NodeConfig.Spec.Storage.Type {
	case v1beta1.KineStorageType:
		storageBackend = &controller.Kine{
			Config:  c.NodeConfig.Spec.Storage.Kine,
			K0sVars: c.K0sVars,
		}
	case v1beta1.EtcdStorageType:
		storageBackend = &controller.Etcd{
			CertManager: certificateManager,
			Config:      c.NodeConfig.Spec.Storage.Etcd,
			JoinClient:  joinClient,
			K0sVars:     c.K0sVars,
			LogLevel:    c.Logging["etcd"],
		}
	default:
		return fmt.Errorf("invalid storage type: %s", c.NodeConfig.Spec.Storage.Type)
	}
	logrus.Infof("using storage backend %s", c.NodeConfig.Spec.Storage.Type)
	c.NodeComponents.Add(ctx, storageBackend)

	// common factory to get the admin kube client that's needed in many components
	adminClientFactory := kubernetes.NewAdminClientFactory(c.K0sVars)
	enableKonnectivity := !c.SingleNode && !slices.Contains(c.DisableComponents, constant.KonnectivityServerComponentName)
	disableEndpointReconciler := !slices.Contains(c.DisableComponents, constant.APIEndpointReconcilerComponentName) &&
		(c.NodeConfig.Spec.API.ExternalAddress != "" || c.NodeConfig.Spec.API.TunneledNetworkingMode)

	c.NodeComponents.Add(ctx, &controller.APIServer{
		ClusterConfig:             c.NodeConfig,
		K0sVars:                   c.K0sVars,
		LogLevel:                  c.Logging["kube-apiserver"],
		Storage:                   storageBackend,
		EnableKonnectivity:        enableKonnectivity,
		DisableEndpointReconciler: disableEndpointReconciler,
	})

	if !c.SingleNode {
		c.NodeComponents.Add(ctx, &controller.K0sControllersLeaseCounter{
			ClusterConfig:     c.NodeConfig,
			KubeClientFactory: adminClientFactory,
		})
	}

	var leaderElector interface {
		leaderelector.Interface
		manager.Component
	}

	// One leader elector per controller
	if !c.SingleNode {
		leaderElector = leaderelector.NewLeasePool(adminClientFactory)
	} else {
		leaderElector = &leaderelector.Dummy{Leader: true}
	}
	c.NodeComponents.Add(ctx, leaderElector)

	c.NodeComponents.Add(ctx, &applier.Manager{
		K0sVars:           c.K0sVars,
		KubeClientFactory: adminClientFactory,
		LeaderElector:     leaderElector,
	})

	if !c.SingleNode && !slices.Contains(c.DisableComponents, constant.ControlAPIComponentName) {
		c.NodeComponents.Add(ctx, &controller.K0SControlAPI{
			ConfigPath: c.CfgFile,
			K0sVars:    c.K0sVars,
		})
	}

	if !slices.Contains(c.DisableComponents, constant.CsrApproverComponentName) {
		c.NodeComponents.Add(ctx, controller.NewCSRApprover(c.NodeConfig,
			leaderElector,
			adminClientFactory))
	}

	if c.EnableK0sCloudProvider {
		c.NodeComponents.Add(
			ctx,
			controller.NewK0sCloudProvider(
				c.K0sVars.AdminKubeConfigPath,
				c.K0sCloudProviderUpdateFrequency,
				c.K0sCloudProviderPort,
			),
		)
	}
	c.NodeComponents.Add(ctx, &status.Status{
		Prober: prober.DefaultProber,
		StatusInformation: status.K0sStatus{
			Pid:           os.Getpid(),
			Role:          "controller",
			Args:          os.Args,
			Version:       build.Version,
			Workloads:     c.SingleNode || c.EnableWorker,
			SingleNode:    c.SingleNode,
			K0sVars:       c.K0sVars,
			ClusterConfig: c.NodeConfig,
		},
		Socket:      config.StatusSocket,
		CertManager: worker.NewCertificateManager(ctx, c.K0sVars.KubeletAuthConfigPath),
	})

	perfTimer.Checkpoint("starting-certificates-init")
	certs := &Certificates{
		ClusterSpec: c.NodeConfig.Spec,
		CertManager: certificateManager,
		K0sVars:     c.K0sVars,
	}
	if err := certs.Init(ctx); err != nil {
		return err
	}

	perfTimer.Checkpoint("starting-node-component-init")
	// init Node components
	if err := c.NodeComponents.Init(ctx); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-node-component-init")

	perfTimer.Checkpoint("starting-node-components")

	// Start components
	err = c.NodeComponents.Start(ctx)
	perfTimer.Checkpoint("finished-starting-node-components")
	if err != nil {
		return fmt.Errorf("failed to start controller node components: %w", err)
	}
	defer func() {
		// Stop components
		if err := c.NodeComponents.Stop(); err != nil {
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
		var clusterConfig *v1beta1.ClusterConfig
		clusterConfig, err = config.LoadClusterConfig(c.K0sVars)
		if err != nil {
			return fmt.Errorf("failed to load cluster config: %w", err)
		}
		configSource, err = clusterconfig.NewStaticSource(clusterConfig)
	}
	if err != nil {
		return err
	}
	defer configSource.Stop()

	if !slices.Contains(c.DisableComponents, constant.APIConfigComponentName) {
		apiConfigSaver, err := controller.NewManifestsSaver("api-config", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize api-config manifests saver: %w", err)
		}

		cfgReconciler, err := controller.NewClusterConfigReconciler(
			leaderElector,
			c.K0sVars,
			c.ClusterComponents,
			apiConfigSaver,
			adminClientFactory,
			configSource,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize cluster-config reconciler: %w", err)
		}
		c.ClusterComponents.Add(ctx, cfgReconciler)
	}

	if !slices.Contains(c.DisableComponents, constant.HelmComponentName) {
		helmSaver, err := controller.NewManifestsSaver("helm", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize helm manifests saver: %w", err)
		}

		c.ClusterComponents.Add(ctx, controller.NewCRD(helmSaver, []string{"helm"}))
		c.ClusterComponents.Add(ctx, controller.NewExtensionsController(
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
		c.ClusterComponents.Add(ctx, controller.NewCRD(manifestsSaver, []string{"autopilot"}))
	}

	if c.NodeConfig.Spec.API.TunneledNetworkingMode {
		c.ClusterComponents.Add(ctx, controller.NewTunneledEndpointReconciler(
			leaderElector,
			adminClientFactory,
		))
	}

	if !slices.Contains(c.DisableComponents, constant.APIEndpointReconcilerComponentName) && c.NodeConfig.Spec.API.ExternalAddress != "" && !c.NodeConfig.Spec.API.TunneledNetworkingMode {
		c.ClusterComponents.Add(ctx, controller.NewEndpointReconciler(
			c.NodeConfig,
			leaderElector,
			adminClientFactory,
			net.DefaultResolver,
		))
	}

	if !slices.Contains(c.DisableComponents, constant.KubeProxyComponentName) {
		c.ClusterComponents.Add(ctx, controller.NewKubeProxy(c.K0sVars, c.NodeConfig))
	}

	if !slices.Contains(c.DisableComponents, constant.CoreDNSComponentname) {
		coreDNS, err := controller.NewCoreDNS(c.K0sVars, adminClientFactory, c.NodeConfig)
		if err != nil {
			return fmt.Errorf("failed to create CoreDNS reconciler: %w", err)
		}
		c.ClusterComponents.Add(ctx, coreDNS)
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
		c.ClusterComponents.Add(ctx, controller.NewCalico(c.K0sVars, calicoInitSaver, calicoSaver))

		kubeRouterSaver, err := controller.NewManifestsSaver("kuberouter", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create kuberouter manifests saver: %w", err)
		}
		c.ClusterComponents.Add(ctx, controller.NewKubeRouter(c.K0sVars, kubeRouterSaver))
	}

	if !slices.Contains(c.DisableComponents, constant.MetricsServerComponentName) {
		c.ClusterComponents.Add(ctx, controller.NewMetricServer(c.K0sVars, adminClientFactory))
	}

	if c.EnableMetricsScraper {
		metricsSaver, err := controller.NewManifestsSaver("metrics", c.K0sVars.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create metrics manifests saver: %w", err)
		}
		metrics, err := controller.NewMetrics(c.K0sVars, metricsSaver, adminClientFactory)
		if err != nil {
			return fmt.Errorf("failed to create metrics reconciler: %w", err)
		}
		c.ClusterComponents.Add(ctx, metrics)
	}

	if !slices.Contains(c.DisableComponents, constant.WorkerConfigComponentName) {
		reconciler, err := workerconfig.NewReconciler(c.K0sVars, c.NodeConfig.Spec, adminClientFactory, leaderElector, enableKonnectivity)
		if err != nil {
			return err
		}
		c.ClusterComponents.Add(ctx, reconciler)
		c.ClusterComponents.Add(ctx, controller.NewKubeletConfig(c.K0sVars, adminClientFactory))
	}

	if !slices.Contains(c.DisableComponents, constant.SystemRbacComponentName) {
		c.ClusterComponents.Add(ctx, controller.NewSystemRBAC(c.K0sVars.ManifestsDir))
	}

	if !slices.Contains(c.DisableComponents, constant.NodeRoleComponentName) {
		c.ClusterComponents.Add(ctx, controller.NewNodeRole(c.K0sVars, adminClientFactory))
	}

	if enableKonnectivity {
		c.ClusterComponents.Add(ctx, &controller.Konnectivity{
			SingleNode:        c.SingleNode,
			LogLevel:          c.Logging[constant.KonnectivityServerComponentName],
			K0sVars:           c.K0sVars,
			KubeClientFactory: adminClientFactory,
			NodeConfig:        c.NodeConfig,
			EventEmitter:      prober.NewEventEmitter(),
		})
	}

	if !slices.Contains(c.DisableComponents, constant.KubeSchedulerComponentName) {
		c.ClusterComponents.Add(ctx, &controller.Scheduler{
			LogLevel:   c.Logging[constant.KubeSchedulerComponentName],
			K0sVars:    c.K0sVars,
			SingleNode: c.SingleNode,
		})
	}

	if !slices.Contains(c.DisableComponents, constant.KubeControllerManagerComponentName) {
		c.ClusterComponents.Add(ctx, &controller.Manager{
			LogLevel:              c.Logging[constant.KubeControllerManagerComponentName],
			K0sVars:               c.K0sVars,
			SingleNode:            c.SingleNode,
			ServiceClusterIPRange: c.NodeConfig.Spec.Network.BuildServiceCIDR(c.NodeConfig.Spec.API.Address),
			ExtraArgs:             c.KubeControllerManagerExtraArgs,
		})
	}

	c.ClusterComponents.Add(ctx, &telemetry.Component{
		Version:           build.Version,
		K0sVars:           c.K0sVars,
		KubeClientFactory: adminClientFactory,
	})

	c.ClusterComponents.Add(ctx, &controller.Autopilot{
		K0sVars:            c.K0sVars,
		AdminClientFactory: adminClientFactory,
		EnableWorker:       c.EnableWorker,
	})

	perfTimer.Checkpoint("starting-cluster-components-init")
	// init Cluster components
	if err := c.ClusterComponents.Init(ctx); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished cluster-component-init")

	err = c.ClusterComponents.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start cluster components: %w", err)
	}
	perfTimer.Checkpoint("finished-starting-cluster-components")
	defer func() {
		// Stop Cluster components
		if err := c.ClusterComponents.Stop(); err != nil {
			logrus.WithError(err).Error("Failed to stop cluster components")
		} else {
			logrus.Info("All cluster components stopped")
		}
	}()

	// At this point all the components should be initialized and running, thus we can release the config for reconcilers
	go configSource.Release(ctx)

	if c.EnableWorker {
		perfTimer.Checkpoint("starting-worker")
		if err := c.startWorker(ctx, c.WorkerProfile); err != nil {
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
	return os.Remove(c.CfgFile)
}

func (c *command) startWorker(ctx context.Context, profile string) error {
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
			cfg, err := token.CreateKubeletBootstrapToken(ctx, c.NodeConfig.Spec.API, c.K0sVars, token.RoleWorker, tokenAge)
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
