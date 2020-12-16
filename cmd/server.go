/*
Copyright 2020 Mirantis, Inc.

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
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/telemetry"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/component/server"
	"github.com/k0sproject/k0s/pkg/component/worker"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/performance"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
)

func init() {
	serverCmd.Flags().StringVar(&serverWorkerProfile, "profile", "default", "worker profile to use on the node")
	serverCmd.Flags().BoolVar(&enableWorker, "enable-worker", false, "enable worker (default false)")
	serverCmd.Flags().StringVar(&tokenFile, "token-file", "", "Path to the file containing join-token.")
}

var (
	serverWorkerProfile string
	enableWorker        bool
	serverToken         string

	serverCmd = &cobra.Command{
		Use:   "server [join-token]",
		Short: "Run server",
		Example: `	Command to associate master nodes:
	CLI argument:
	$ k0s server [join-token]

	or CLI flag:
	$ k0s server --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				serverToken = args[0]
			}
			if len(serverToken) > 0 && len(tokenFile) > 0 {
				return fmt.Errorf("You can only pass one token argument either as a CLI argument 'k0s server [join-token]' or as a flag 'k0s server --token-file [path]'")
			}

			if len(tokenFile) > 0 {
				bytes, err := ioutil.ReadFile(tokenFile)
				if err != nil {
					return err
				}
				serverToken = string(bytes)
			}
			return startServer(serverToken)
		},
	}
)

// If we've got CA in place we assume the node has already joined previously
func needToJoin(k0sVars constant.CfgVars) bool {
	if util.FileExists(filepath.Join(k0sVars.CertRootDir, "ca.key")) &&
		util.FileExists(filepath.Join(k0sVars.CertRootDir, "ca.crt")) {
		return false
	}
	return true
}

func startServer(token string) error {
	perfTimer := performance.NewTimer("server-start").Buffer().Start()
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
	if token != "" && needToJoin(k0sVars) {
		join = true
		joinClient, err = v1beta1.JoinClientFromToken(token)
		if err != nil {
			return errors.Wrapf(err, "failed to create join client")
		}

		componentManager.AddSync(&server.CASyncer{
			JoinClient: joinClient,
			K0sVars:    k0sVars,
		})
	}
	componentManager.AddSync(&server.Certificates{
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
	case v1beta1.KineStorageType, "":
		storageBackend = &server.Kine{
			Config:  clusterConfig.Spec.Storage.Kine,
			K0sVars: k0sVars,
		}
	case v1beta1.EtcdStorageType:
		storageBackend = &server.Etcd{
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

	componentManager.Add(&server.APIServer{
		ClusterConfig: clusterConfig,
		K0sVars:       k0sVars,
		LogLevel:      logging["kube-apiserver"],
		Storage:       storageBackend,
	})
	componentManager.Add(&server.Konnectivity{
		ClusterConfig: clusterConfig,
		LogLevel:      logging["konnectivity-server"],
		K0sVars:       k0sVars,
	})
	componentManager.Add(&server.Scheduler{
		ClusterConfig: clusterConfig,
		LogLevel:      logging["kube-scheduler"],
		K0sVars:       k0sVars,
	})
	componentManager.Add(&server.ControllerManager{
		ClusterConfig: clusterConfig,
		LogLevel:      logging["kube-controller-manager"],
		K0sVars:       k0sVars,
	})
	componentManager.Add(&applier.Manager{K0sVars: k0sVars})
	componentManager.Add(&server.K0SControlAPI{
		ConfigPath: cfgFile,
		K0sVars:    k0sVars,
	})

	if clusterConfig.Telemetry.Enabled {
		componentManager.Add(&telemetry.Component{
			ClusterConfig: clusterConfig,
			Version:       build.Version,
			K0sVars:       k0sVars,
		})
	}

	perfTimer.Checkpoint("starting-component-init")
	// init components
	if err := componentManager.Init(); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-component-init")

	// Set up signal handling. Use buffered channel so we dont miss
	// signals during startup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	perfTimer.Checkpoint("starting-components")
	// Start components
	err = componentManager.Start()
	perfTimer.Checkpoint("finished-starting-components")
	if err != nil {
		logrus.Errorf("failed to start server components: %s", err)
		c <- syscall.SIGTERM
	}

	// in-cluster component reconcilers
	reconcilers := createClusterReconcilers(clusterConfig, k0sVars)
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
		err = enableServerWorker(clusterConfig, k0sVars, componentManager, serverWorkerProfile)
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
	<-c
	logrus.Info("Shutting down k0s server")

	// Stop all reconcilers first
	for _, reconciler := range reconcilers {
		if err := reconciler.Stop(); err != nil {
			logrus.Warningf("failed to stop reconciler: %s", err.Error())
		}
	}

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.Errorf("error while stoping component manager %s", err)
	}
	return nil
}

func createClusterReconcilers(clusterConf *config.ClusterConfig, k0sVars constant.CfgVars) map[string]component.Component {
	reconcilers := make(map[string]component.Component)
	clusterSpec := clusterConf.Spec

	defaultPSP, err := server.NewDefaultPSP(clusterSpec, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize default PSP reconciler: %s", err.Error())
	} else {
		reconcilers["default-psp"] = defaultPSP
	}

	proxy, err := server.NewKubeProxy(clusterConf, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := server.NewCoreDNS(clusterConf, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	initNetwork(reconcilers, clusterConf, k0sVars.DataDir)

	manifestsSaver, err := server.NewManifestsSaver("helm", k0sVars.DataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	reconcilers["crd"] = server.NewCRD(manifestsSaver)
	reconcilers["helmAddons"] = server.NewHelmAddons(clusterConf, manifestsSaver, k0sVars)

	metricServer, err := server.NewMetricServer(clusterConf, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize metric server reconciler: %s", err.Error())
	} else {
		reconcilers["metricServer"] = metricServer
	}

	kubeletConfig, err := server.NewKubeletConfig(clusterSpec, k0sVars)
	if err != nil {
		logrus.Warnf("failed to initialize kubelet config reconciler: %s", err.Error())
	} else {
		reconcilers["kubeletConfig"] = kubeletConfig
	}

	systemRBAC, err := server.NewSystemRBAC(k0sVars.ManifestsDir)
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
	saver, err := server.NewManifestsSaver("calico", dataDir)
	if err != nil {
		logrus.Warnf("failed to initialize reconcilers manifests saver: %s", err.Error())
	}
	calico, err := server.NewCalico(conf, saver)

	if err != nil {
		logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
		return
	}

	reconcilers["calico"] = calico

}

func enableServerWorker(clusterConfig *config.ClusterConfig, k0sVars constant.CfgVars, componentManager *component.Manager, profile string) error {
	if !util.FileExists(k0sVars.KubeletAuthConfigPath) {
		// wait for server to start up
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
			config, err := createKubeletBootstrapConfig(clusterConfig, "worker", time.Minute)
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

	containerd := &worker.ContainerD{
		LogLevel: logging["containerd"],
		K0sVars:  k0sVars,
	}
	kubelet := &worker.Kubelet{
		KubeletConfigClient: kubeletConfigClient,
		Profile:             profile,
		LogLevel:            logging["kubelet"],
		K0sVars:             k0sVars,
	}

	if err := containerd.Init(); err != nil {
		logrus.Errorf("failed to init containerd: %s", err)
	}
	if err := kubelet.Init(); err != nil {
		logrus.Errorf("failed to init kubelet: %s", err)
	}
	if err := containerd.Run(); err != nil {
		logrus.Errorf("failed to run containerd: %s", err)
	}
	if err := kubelet.Run(); err != nil {
		logrus.Errorf("failed to run kubelet: %s", err)
	}

	componentManager.Add(containerd)
	componentManager.Add(kubelet)

	return nil
}
