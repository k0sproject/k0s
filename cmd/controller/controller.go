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
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/performance"
	"github.com/k0sproject/k0s/pkg/telemetry"
	"github.com/k0sproject/k0s/pkg/token"
)

type CmdOpts config.CLIOptions

func NewControllerCmd() *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			c := CmdOpts(config.GetCmdOpts())
			if len(args) > 0 {
				c.TokenArg = args[0]
			}
			if len(c.TokenArg) > 0 && len(c.TokenFile) > 0 {
				return fmt.Errorf("You can only pass one token argument either as a CLI argument 'k0s controller [join-token]' or as a flag 'k0s controller --token-file [path]'")
			}
			if len(c.TokenFile) > 0 {
				bytes, err := ioutil.ReadFile(c.TokenFile)
				if err != nil {
					return err
				}
				c.TokenArg = string(bytes)
			}
			if c.SingleNode {
				c.EnableWorker = true
				c.K0sVars.DefaultStorageType = "kine"
			}
			c.Logging = util.MapMerge(c.CmdLogLevels, c.DefaultLogLevels)
			cfg, err := config.GetYamlFromFile(c.CfgFile, c.K0sVars)
			if err != nil {
				return err
			}

			c.ClusterConfig = cfg
			cmd.SilenceUsage = true
			return c.startController()
		},
	}

	// append flags
	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())
	cmd.PersistentFlags().AddFlagSet(config.GetControllerFlags())
	cmd.PersistentFlags().AddFlagSet(config.GetWorkerFlags())
	return cmd
}

// If we've got CA in place we assume the node has already joined previously
func (c *CmdOpts) needToJoin() bool {
	if util.FileExists(filepath.Join(c.K0sVars.CertRootDir, "ca.key")) &&
		util.FileExists(filepath.Join(c.K0sVars.CertRootDir, "ca.crt")) {
		return false
	}
	return true
}

func (c *CmdOpts) startController() error {
	perfTimer := performance.NewTimer("controller-start").Buffer().Start()

	// create directories early with the proper permissions
	if err := util.InitDirectory(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
		return err
	}
	if err := util.InitDirectory(c.K0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}

	componentManager := component.NewManager()
	certificateManager := certificate.Manager{K0sVars: c.K0sVars}

	var join = false

	var joinClient *token.JoinClient
	var err error

	if c.TokenArg != "" && c.needToJoin() {
		join = true
		joinClient, err = token.JoinClientFromToken(c.TokenArg)
		if err != nil {
			return errors.Wrapf(err, "failed to create join client")
		}

		componentManager.AddSync(&controller.CASyncer{
			JoinClient: joinClient,
			K0sVars:    c.K0sVars,
		})
	}
	componentManager.AddSync(&controller.Certificates{
		ClusterSpec: c.ClusterConfig.Spec,
		CertManager: certificateManager,
		K0sVars:     c.K0sVars,
	})

	logrus.Infof("using public address: %s", c.ClusterConfig.Spec.API.Address)
	logrus.Infof("using sans: %s", c.ClusterConfig.Spec.API.SANs)
	dnsAddress, err := c.ClusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)
	var storageBackend component.Component

	switch c.ClusterConfig.Spec.Storage.Type {
	case v1beta1.KineStorageType:
		storageBackend = &controller.Kine{
			Config:  c.ClusterConfig.Spec.Storage.Kine,
			K0sVars: c.K0sVars,
		}
	case v1beta1.EtcdStorageType:
		storageBackend = &controller.Etcd{
			CertManager: certificateManager,
			Config:      c.ClusterConfig.Spec.Storage.Etcd,
			Join:        join,
			JoinClient:  joinClient,
			K0sVars:     c.K0sVars,
			LogLevel:    c.Logging["etcd"],
		}
	default:
		return errors.New(fmt.Sprintf("Invalid storage type: %s", c.ClusterConfig.Spec.Storage.Type))
	}
	logrus.Infof("Using storage backend %s", c.ClusterConfig.Spec.Storage.Type)
	componentManager.Add(storageBackend)

	// common factory to get the admin kube client that's needed in many components
	adminClientFactory := kubernetes.NewAdminClientFactory(c.K0sVars)

	componentManager.Add(&controller.APIServer{
		ClusterConfig:      c.ClusterConfig,
		K0sVars:            c.K0sVars,
		LogLevel:           c.Logging["kube-apiserver"],
		Storage:            storageBackend,
		EnableKonnectivity: !c.SingleNode,
	})

	if c.ClusterConfig.Spec.API.ExternalAddress != "" {
		componentManager.Add(&controller.K0sLease{
			ClusterConfig:     c.ClusterConfig,
			KubeClientFactory: adminClientFactory,
		})
	}
	if !c.SingleNode {
		componentManager.Add(&controller.Konnectivity{
			ClusterConfig:     c.ClusterConfig,
			LogLevel:          c.Logging["konnectivity-server"],
			K0sVars:           c.K0sVars,
			KubeClientFactory: adminClientFactory,
		})
	}
	componentManager.Add(&controller.Scheduler{
		ClusterConfig: c.ClusterConfig,
		LogLevel:      c.Logging["kube-scheduler"],
		K0sVars:       c.K0sVars,
	})
	componentManager.Add(&controller.Manager{
		ClusterConfig: c.ClusterConfig,
		LogLevel:      c.Logging["kube-controller-manager"],
		K0sVars:       c.K0sVars,
	})

	// One leader elector per controller
	var leaderElector controller.LeaderElector
	if c.ClusterConfig.Spec.API.ExternalAddress != "" {
		leaderElector = controller.NewLeaderElector(c.ClusterConfig, adminClientFactory)
	} else {
		leaderElector = &controller.DummyLeaderElector{Leader: true}
	}
	componentManager.Add(leaderElector)

	componentManager.Add(&applier.Manager{K0sVars: c.K0sVars, KubeClientFactory: adminClientFactory, LeaderElector: leaderElector})
	if !c.SingleNode {
		componentManager.Add(&controller.K0SControlAPI{
			ConfigPath: c.CfgFile,
			K0sVars:    c.K0sVars,
		})
	}
	if c.ClusterConfig.Spec.Telemetry.Enabled {
		componentManager.Add(&telemetry.Component{
			ClusterConfig:     c.ClusterConfig,
			Version:           build.Version,
			K0sVars:           c.K0sVars,
			KubeClientFactory: adminClientFactory,
		})
	}

	if c.ClusterConfig.Spec.API.ExternalAddress != "" {
		componentManager.Add(controller.NewEndpointReconciler(
			c.ClusterConfig,
			leaderElector,
			adminClientFactory,
		))
	}

	componentManager.Add(controller.NewCSRApprover(c.ClusterConfig,
		leaderElector,
		adminClientFactory))

	perfTimer.Checkpoint("starting-component-init")
	// init components
	if err := componentManager.Init(); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-component-init")

	// Set up signal handling. Use buffered channel so we dont miss
	// signals during startup
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(ch)
		cancel()
	}()

	go func() {
		select {
		case <-ch:
			logrus.Info("Shutting down k0s controller")
			cancel()
		case <-ctx.Done():
			logrus.Debug("Context done in go-routine")
		}
	}()

	perfTimer.Checkpoint("starting-components")
	// Start components
	err = componentManager.Start(ctx)
	perfTimer.Checkpoint("finished-starting-components")
	if err != nil {
		logrus.Errorf("failed to start controller components: %s", err)
		ch <- syscall.SIGTERM
	}

	// in-cluster component reconcilers
	reconcilers := c.createClusterReconcilers(adminClientFactory, leaderElector)
	if err == nil {
		perfTimer.Checkpoint("starting-reconcilers")

		// Start all reconcilers
		for _, reconciler := range reconcilers {
			if err := reconciler.Run(); err != nil {
				logrus.Errorf("failed to start reconciler: %s", err.Error())
			}
		}
	}
	perfTimer.Checkpoint("started-reconcilers")

	if err == nil && c.EnableWorker {
		perfTimer.Checkpoint("starting-worker")

		err = c.startControllerWorker(ctx, c.WorkerProfile)
		if err != nil {
			logrus.Errorf("failed to start worker components: %s", err)
			if err := componentManager.Stop(); err != nil {
				logrus.Errorf("componentManager.Stop: %s", err)
			}
			return err
		}
		perfTimer.Checkpoint("started-worker")
	}

	perfTimer.Output()

	// Wait for k0s process termination
	<-ctx.Done()
	logrus.Debug("Context done in main")

	// Stop all reconcilers first
	for _, reconciler := range reconcilers {
		if err := reconciler.Stop(); err != nil {
			logrus.Warningf("failed to stop reconciler: %s", err.Error())
		}
	}

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.Errorf("error while stopping component manager %s", err)
	}
	return nil
}

func (c *CmdOpts) createClusterReconcilers(cf kubernetes.ClientFactory, leaderElector controller.LeaderElector) map[string]component.Component {
	reconcilers := make(map[string]component.Component)
	clusterSpec := c.ClusterConfig.Spec

	defaultPSP, err := controller.NewDefaultPSP(clusterSpec, c.K0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize default PSP reconciler: %s", err.Error())
	} else {
		reconcilers["default-psp"] = defaultPSP
	}

	proxy, err := controller.NewKubeProxy(c.ClusterConfig, c.K0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := controller.NewCoreDNS(c.ClusterConfig, c.K0sVars, cf)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	c.initNetwork(reconcilers)

	manifestsSaver, err := controller.NewManifestsSaver("helm", c.K0sVars.DataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	reconcilers["crd"] = controller.NewCRD(manifestsSaver)
	reconcilers["helmAddons"] = controller.NewHelmAddons(c.ClusterConfig, manifestsSaver, c.K0sVars, cf, leaderElector)

	metricServer, err := controller.NewMetricServer(c.ClusterConfig, c.K0sVars, cf)
	if err != nil {
		logrus.Warnf("failed to initialize metric controller reconciler: %s", err.Error())
	} else {
		reconcilers["metricServer"] = metricServer
	}

	kubeletConfig, err := controller.NewKubeletConfig(clusterSpec, c.K0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kubelet config reconciler: %s", err.Error())
	} else {
		reconcilers["kubeletConfig"] = kubeletConfig
	}

	systemRBAC, err := controller.NewSystemRBAC(c.K0sVars.ManifestsDir)
	if err != nil {
		logrus.Warnf("failed to initialize system RBAC reconciler: %s", err.Error())
	} else {
		reconcilers["systemRBAC"] = systemRBAC
	}

	return reconcilers
}

func (c *CmdOpts) initNetwork(reconcilers map[string]component.Component) {
	if c.ClusterConfig.Spec.Network.Provider != "calico" {
		logrus.Warnf("network provider set to custom, k0s will not manage it")
		return
	}
	calicoSaver, err := controller.NewManifestsSaver("calico", c.K0sVars.DataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	calicoInitSaver, err := controller.NewManifestsSaver("calico_init", c.K0sVars.DataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	calico, err := controller.NewCalico(c.ClusterConfig, calicoInitSaver, calicoSaver)

	if err != nil {
		logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
		return
	}
	reconcilers["calico"] = calico
}

func (c *CmdOpts) startControllerWorker(ctx context.Context, profile string) error {
	// we use separate controllerManager here
	// because we need to have controllers components
	// be fully initialized and running before running worker components

	workerComponentManager := component.NewManager()
	if !util.FileExists(c.K0sVars.KubeletAuthConfigPath) {
		// wait for controller to start up
		err := retry.Do(func() error {
			if !util.FileExists(c.K0sVars.AdminKubeConfigPath) {
				return fmt.Errorf("file does not exist: %s", c.K0sVars.AdminKubeConfigPath)
			}
			return nil
		})
		if err != nil {
			return err
		}

		var bootstrapConfig string
		err = retry.Do(func() error {
			// five minutes here are coming from maximum theoretical duration of kubelet bootstrap process
			// we use retry.Do with 10 attempts, back-off delay and delay duration 500 ms which gives us
			// 225 seconds here
			tokenAge := time.Second * 225
			config, err := token.CreateKubeletBootstrapConfig(c.ClusterConfig, c.K0sVars, "worker", tokenAge)

			if err != nil {
				return err
			}
			bootstrapConfig = config
			return nil
		})
		if err != nil {
			return err
		}
		if err := worker.HandleKubeletBootstrapToken(bootstrapConfig, c.K0sVars); err != nil {
			return err
		}
	}
	worker.KernelSetup()
	kubeletConfigClient, err := worker.LoadKubeletConfigClient(c.K0sVars)
	if err != nil {
		return err
	}

	workerComponentManager.Add(&worker.ContainerD{
		LogLevel: c.Logging["containerd"],
		K0sVars:  c.K0sVars,
	})
	workerComponentManager.Add(worker.NewOCIBundleReconciler(c.K0sVars))
	workerComponentManager.Add(&worker.Kubelet{
		CRISocket:           c.CriSocket,
		EnableCloudProvider: c.CloudProvider,
		K0sVars:             c.K0sVars,
		KubeletConfigClient: kubeletConfigClient,
		LogLevel:            c.Logging["kubelet"],
		Profile:             c.WorkerProfile,
		Labels:              c.Labels,
		ExtraArgs:           c.KubeletExtraArgs,
	})

	if err := workerComponentManager.Init(); err != nil {
		return fmt.Errorf("can't init worker components: %w", err)
	}
	go func() {
		<-ctx.Done()
		if err := workerComponentManager.Stop(); err != nil {
			logrus.WithError(err).Error("can't properly stop worker components")
		}
	}()
	if err := workerComponentManager.Start(ctx); err != nil {
		return fmt.Errorf("can't start worker components: %w", err)
	}
	return nil
}
