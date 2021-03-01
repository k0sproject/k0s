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
package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/telemetry"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/performance"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
)

func init() {
	controllerCmd.Flags().StringVar(&controllerWorkerProfile, "profile", "default", "worker profile to use on the node")
	controllerCmd.Flags().BoolVar(&enableWorker, "enable-worker", false, "enable worker (default false)")
	controllerCmd.Flags().BoolVar(&singleNode, "single", false, "enable single node (implies --enable-worker, default false)")
	controllerCmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to the file containing join-token.")
	controllerCmd.Flags().StringVar(&criSocket, "cri-socket", "", "contrainer runtime socket to use, default to internal containerd. Format: [remote|docker]:[path-to-socket]")
	controllerCmd.Flags().StringToStringVarP(&cmdLogLevels, "logging", "l", defaultLogLevels, "Logging Levels for the different components")
	addPersistentFlags(controllerCmd)
	installControllerCmd.Flags().AddFlagSet(controllerCmd.Flags())
}

var (
	controllerWorkerProfile string
	enableWorker            bool
	singleNode              bool
	controllerToken         string
	controllerCmd           = &cobra.Command{
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
			if len(args) > 0 {
				controllerToken = args[0]
			}
			if len(controllerToken) > 0 && len(tokenFile) > 0 {
				return fmt.Errorf("You can only pass one token argument either as a CLI argument 'k0s controller [join-token]' or as a flag 'k0s controller --token-file [path]'")
			}

			if len(tokenFile) > 0 {
				bytes, err := ioutil.ReadFile(tokenFile)
				if err != nil {
					return err
				}
				controllerToken = string(bytes)
			}
			if singleNode {
				enableWorker = true
				k0sVars.DefaultStorageType = "kine"
			}

			return startController(controllerToken)
		},
	}
)

// If we've got CA in place we assume the node has already joined previously
func needToJoin() bool {
	if util.FileExists(filepath.Join(k0sVars.CertRootDir, "ca.key")) &&
		util.FileExists(filepath.Join(k0sVars.CertRootDir, "ca.crt")) {
		return false
	}
	return true
}

func startController(token string) error {
	perfTimer := performance.NewTimer("controller-start").Buffer().Start()
	clusterConfig, err := ConfigFromYaml(cfgFile)
	if err != nil {
		return err
	}

	// create directories early with the proper permissions
	if err = util.InitDirectory(k0sVars.DataDir, constant.DataDirMode); err != nil {
		return err
	}
	if err := util.InitDirectory(k0sVars.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}

	componentManager := component.NewManager()
	certificateManager := certificate.Manager{K0sVars: k0sVars}

	var join = false

	var joinClient *v1beta1.JoinClient
	if token != "" && needToJoin() {
		join = true
		joinClient, err = v1beta1.JoinClientFromToken(token)
		if err != nil {
			return errors.Wrapf(err, "failed to create join client")
		}

		componentManager.AddSync(&controller.CASyncer{
			JoinClient: joinClient,
			K0sVars:    k0sVars,
		})
	}
	componentManager.AddSync(&controller.Certificates{
		ClusterSpec: clusterConfig.Spec,
		CertManager: certificateManager,
		K0sVars:     k0sVars,
	})

	logrus.Infof("using public address: %s", clusterConfig.Spec.API.Address)
	logrus.Infof("using sans: %s", clusterConfig.Spec.API.SANs)
	dnsAddress, err := clusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)
	var storageBackend component.Component

	switch clusterConfig.Spec.Storage.Type {
	case v1beta1.KineStorageType:
		storageBackend = &controller.Kine{
			Config:  clusterConfig.Spec.Storage.Kine,
			K0sVars: k0sVars,
		}
	case v1beta1.EtcdStorageType:
		storageBackend = &controller.Etcd{
			CertManager: certificateManager,
			Config:      clusterConfig.Spec.Storage.Etcd,
			Join:        join,
			JoinClient:  joinClient,
			K0sVars:     k0sVars,
			LogLevel:    logging["etcd"],
		}
	default:
		return errors.New(fmt.Sprintf("Invalid storage type: %s", clusterConfig.Spec.Storage.Type))
	}
	logrus.Infof("Using storage backend %s", clusterConfig.Spec.Storage.Type)
	componentManager.Add(storageBackend)

	// common factory to get the admin kube client that's needed in many components
	adminClientFactory := kubernetes.NewAdminClientFactory(k0sVars)

	componentManager.Add(&controller.APIServer{
		ClusterConfig: clusterConfig,
		K0sVars:       k0sVars,
		LogLevel:      logging["kube-apiserver"],
		Storage:       storageBackend,
	})

	if clusterConfig.Spec.API.ExternalAddress != "" {
		componentManager.Add(&controller.K0sLease{
			ClusterConfig:     clusterConfig,
			KubeClientFactory: adminClientFactory,
		})
	}

	if !singleNode {
		componentManager.Add(&controller.Konnectivity{
			ClusterConfig:     clusterConfig,
			LogLevel:          logging["konnectivity-server"],
			K0sVars:           k0sVars,
			KubeClientFactory: adminClientFactory,
		})
	}
	componentManager.Add(&controller.Scheduler{
		ClusterConfig: clusterConfig,
		LogLevel:      logging["kube-scheduler"],
		K0sVars:       k0sVars,
	})
	componentManager.Add(&controller.Manager{
		ClusterConfig: clusterConfig,
		LogLevel:      logging["kube-controller-manager"],
		K0sVars:       k0sVars,
	})

	// One leader elector per controller
	var leaderElector controller.LeaderElector
	if clusterConfig.Spec.API.ExternalAddress != "" {
		leaderElector = controller.NewLeaderElector(clusterConfig, adminClientFactory)
	} else {
		leaderElector = &controller.DummyLeaderElector{Leader: true}
	}
	componentManager.Add(leaderElector)

	componentManager.Add(&applier.Manager{K0sVars: k0sVars, KubeClientFactory: adminClientFactory, LeaderElector: leaderElector})
	componentManager.Add(&controller.K0SControlAPI{
		ConfigPath: cfgFile,
		K0sVars:    k0sVars,
	})

	if clusterConfig.Spec.Telemetry.Enabled {
		componentManager.Add(&telemetry.Component{
			ClusterConfig:     clusterConfig,
			Version:           build.Version,
			K0sVars:           k0sVars,
			KubeClientFactory: adminClientFactory,
		})
	}

	if clusterConfig.Spec.API.ExternalAddress != "" {
		componentManager.Add(controller.NewEndpointReconciler(
			clusterConfig,
			leaderElector,
			adminClientFactory,
		))
	}

	componentManager.Add(controller.NewCSRApprover(clusterConfig,
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(c)
		cancel()
	}()

	go func() {
		select {
		case <-c:
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
		c <- syscall.SIGTERM
	}

	// in-cluster component reconcilers
	reconcilers := createClusterReconcilers(clusterConfig, k0sVars, adminClientFactory, leaderElector)
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

	if err == nil && enableWorker {
		perfTimer.Checkpoint("starting-worker")
		err = startControllerWorker(ctx, clusterConfig, k0sVars, controllerWorkerProfile)
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

func createClusterReconcilers(clusterConf *config.ClusterConfig, k0sVars constant.CfgVars, cf kubernetes.ClientFactory, leaderElector controller.LeaderElector) map[string]component.Component {
	reconcilers := make(map[string]component.Component)
	clusterSpec := clusterConf.Spec

	defaultPSP, err := controller.NewDefaultPSP(clusterSpec, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize default PSP reconciler: %s", err.Error())
	} else {
		reconcilers["default-psp"] = defaultPSP
	}

	proxy, err := controller.NewKubeProxy(clusterConf, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := controller.NewCoreDNS(clusterConf, k0sVars, cf)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	initNetwork(reconcilers, clusterConf, k0sVars.DataDir)

	manifestsSaver, err := controller.NewManifestsSaver("helm", k0sVars.DataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	reconcilers["crd"] = controller.NewCRD(manifestsSaver)
	reconcilers["helmAddons"] = controller.NewHelmAddons(clusterConf, manifestsSaver, k0sVars, cf, leaderElector)

	metricServer, err := controller.NewMetricServer(clusterConf, k0sVars, cf)
	if err != nil {
		logrus.Warnf("failed to initialize metric controller reconciler: %s", err.Error())
	} else {
		reconcilers["metricServer"] = metricServer
	}

	kubeletConfig, err := controller.NewKubeletConfig(clusterSpec, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kubelet config reconciler: %s", err.Error())
	} else {
		reconcilers["kubeletConfig"] = kubeletConfig
	}

	systemRBAC, err := controller.NewSystemRBAC(k0sVars.ManifestsDir)
	if err != nil {
		logrus.Warnf("failed to initialize system RBAC reconciler: %s", err.Error())
	} else {
		reconcilers["systemRBAC"] = systemRBAC
	}

	return reconcilers
}

func initNetwork(reconcilers map[string]component.Component, conf *config.ClusterConfig, dataDir string) {

	if conf.Spec.Network.Provider != "calico" {
		logrus.Warnf("network provider set to custom, k0s will not manage it")
		return
	}
	calicoSaver, err := controller.NewManifestsSaver("calico", dataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	calicoInitSaver, err := controller.NewManifestsSaver("calico_init", dataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	calico, err := controller.NewCalico(conf, calicoInitSaver, calicoSaver)

	if err != nil {
		logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
		return
	}

	reconcilers["calico"] = calico

}

func startControllerWorker(ctx context.Context, clusterConfig *config.ClusterConfig, k0sVars constant.CfgVars, profile string) error {
	// we use separate controllerManager here
	// because we need to have controllers components
	// be fully initialized and running before running worker components

	workerComponentManager := component.NewManager()
	if !util.FileExists(k0sVars.KubeletAuthConfigPath) {
		// wait for controller to start up
		err := retry.Do(func() error {
			if !util.FileExists(k0sVars.AdminKubeConfigPath) {
				return fmt.Errorf("file does not exist: %s", k0sVars.AdminKubeConfigPath)
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
			config, err := createKubeletBootstrapConfig(clusterConfig, "worker", tokenAge)

			if err != nil {
				return err
			}
			bootstrapConfig = config

			return nil
		})

		if err != nil {
			return err
		}
		if err := handleKubeletBootstrapToken(bootstrapConfig, k0sVars); err != nil {
			return err
		}
	}
	worker.KernelSetup()

	kubeletConfigClient, err := loadKubeletConfigClient(k0sVars)
	if err != nil {
		return err
	}

	workerComponentManager.Add(&worker.ContainerD{
		LogLevel: logging["containerd"],
		K0sVars:  k0sVars,
	})
	workerComponentManager.Add(worker.NewOCIBundleReconciler(k0sVars))
	workerComponentManager.Add(&worker.Kubelet{
		CRISocket:           criSocket,
		KubeletConfigClient: kubeletConfigClient,
		Profile:             profile,
		LogLevel:            logging["kubelet"],
		K0sVars:             k0sVars,
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
