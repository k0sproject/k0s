package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Mirantis/mke/pkg/performance"

	"github.com/Mirantis/mke/pkg/applier"
	"github.com/Mirantis/mke/pkg/certificate"
	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/component/server"
	"github.com/Mirantis/mke/pkg/component/worker"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// ServerCommand ...
func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:   "server",
		Usage:  "Run server",
		Action: startServer,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "mke.yaml",
			},
			&cli.BoolFlag{
				Name:  "enable-worker",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "profile",
				Value: "default",
				Usage: "worker profile to use on the node",
			},
		},
		ArgsUsage: "[join-token]",
	}
}

func configFromCmdFlag(ctx *cli.Context) (*config.ClusterConfig, error) {
	clusterConfig := ConfigFromYaml(ctx)

	errors := clusterConfig.Validate()
	if len(errors) > 0 {
		messages := make([]string, len(errors))
		for _, e := range errors {
			messages = append(messages, e.Error())
		}
		return nil, fmt.Errorf("config yaml does not pass validation, following errors found:%s", strings.Join(messages, "\n"))
	}

	return clusterConfig, nil
}

func startServer(ctx *cli.Context) error {
	perfTimer := performance.NewTimer("server-start").Buffer().Start()
	clusterConfig, err := configFromCmdFlag(ctx)
	if err != nil {
		return err
	}
	componentManager := component.NewManager()
	if err := util.InitDirectory(constant.CertRootDir, constant.CertRootDirMode); err != nil {
		return err
	}
	certificateManager := certificate.Manager{}

	var join = false
	var joinClient *v1beta1.JoinClient
	token := ctx.Args().First()
	if token != "" {
		perfTimer.Checkpoint("token-join-start")
		join = true
		joinClient, err = v1beta1.JoinClientFromToken(token)
		if err != nil {
			return errors.Wrapf(err, "failed to create join client")
		}
		caSyncer := &server.CASyncer{
			JoinClient: joinClient,
		}

		err = caSyncer.Init()
		if err != nil {
			logrus.Warnf("something failed in CA sync: %s", err.Error())
		}
		err = caSyncer.Run()
		if err != nil {
			return errors.Wrapf(err, "CA sync failed")
		}
		perfTimer.Checkpoint("token-join-completed")
	}

	logrus.Infof("using public address: %s", clusterConfig.Spec.API.Address)
	logrus.Infof("using sans: %s", clusterConfig.Spec.API.SANs)
	dnsAddress, err := clusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)

	switch clusterConfig.Spec.Storage.Type {
	case v1beta1.KineStorageType, "":
		componentManager.Add(&server.Kine{
			Config: clusterConfig.Spec.Storage.Kine,
		})
	case v1beta1.EtcdStorageType:
		componentManager.Add(&server.Etcd{
			Config:      clusterConfig.Spec.Storage.Etcd,
			Join:        join,
			CertManager: certificateManager,
			JoinClient:  joinClient,
		})
	default:
		return errors.New(fmt.Sprintf("Invalid storage type: %s", clusterConfig.Spec.Storage.Type))
	}
	logrus.Infof("Using storage backend %s", clusterConfig.Spec.Storage.Type)

	componentManager.Add(&server.APIServer{
		ClusterConfig: clusterConfig,
	})
	componentManager.Add(&server.Konnectivity{
		ClusterConfig: clusterConfig,
	})
	componentManager.Add(&server.Scheduler{
		ClusterConfig: clusterConfig,
	})
	componentManager.Add(&server.ControllerManager{
		ClusterConfig: clusterConfig,
	})
	componentManager.Add(&applier.Manager{})
	componentManager.Add(&server.MkeControlAPI{
		ConfigPath: ctx.String("config"),
	})

	perfTimer.Checkpoint("starting-component-init")
	// init components
	if err := componentManager.Init(); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-component-init")

	certs := server.Certificates{
		ClusterSpec: clusterConfig.Spec,
		CertManager: certificateManager,
	}

	perfTimer.Checkpoint("starting-cert-run")
	if err := certs.Run(); err != nil {
		return err
	}
	perfTimer.Checkpoint("finished-cert-run")

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

	perfTimer.Checkpoint("starting-reconcilers")
	// in-cluster component reconcilers
	reconcilers := createClusterReconcilers(clusterConfig)
	if err == nil {
		// Start all reconcilers
		for _, reconciler := range reconcilers {
			if err := reconciler.Run(); err != nil {
				logrus.Errorf("failed to start reconciler: %s", err.Error())
			}
		}
	}
	perfTimer.Checkpoint("started-reconcilers")

	if err == nil && ctx.Bool("enable-worker") {
		perfTimer.Checkpoint("starting-worker")
		err = enableServerWorker(clusterConfig, componentManager, ctx.String("profile"))
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

	// Wait for mke process termination
	<-c
	logrus.Info("Shutting down mke server")

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

func createClusterReconcilers(clusterConf *config.ClusterConfig) map[string]component.Component {
	reconcilers := make(map[string]component.Component)
	clusterSpec := clusterConf.Spec

	defaultPSP, err := server.NewDefaultPSP(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize default PSP reconciler: %s", err.Error())
	} else {
		reconcilers["default-psp"] = defaultPSP
	}

	proxy, err := server.NewKubeProxy(clusterConf)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := server.NewCoreDNS(clusterConf)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	if clusterSpec.Network.Provider == "calico" {
		calico, err := server.NewCalico(clusterConf)
		if err != nil {
			logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
		} else {
			reconcilers["calico"] = calico
		}
	} else {
		logrus.Warnf("network provider set to custom, mke will not manage it")
	}

	metricServer, err := server.NewMetricServer(clusterConf)
	if err != nil {
		logrus.Warnf("failed to initialize metric server reconciler: %s", err.Error())
	} else {
		reconcilers["metricServer"] = metricServer
	}

	kubeletConfig, err := server.NewKubeletConfig(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize kubelet config reconciler: %s", err.Error())
	} else {
		reconcilers["kubeletConfig"] = kubeletConfig
	}

	systemRBAC, err := server.NewSystemRBAC(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize system RBAC reconciler: %s", err.Error())
	} else {
		reconcilers["systemRBAC"] = systemRBAC
	}

	return reconcilers
}

func enableServerWorker(clusterConfig *config.ClusterConfig, componentManager *component.Manager, profile string) error {
	if !util.FileExists(constant.KubeletAuthConfigPath) {
		// wait for server to start up
		err := retry.Do(func() error {
			if !util.FileExists(constant.AdminKubeconfigConfigPath) {
				return fmt.Errorf("file does not exist: %s", constant.AdminKubeconfigConfigPath)
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
		if err := handleKubeletBootstrapToken(bootstrapConfig); err != nil {
			return err
		}
	}
	worker.KernelSetup()

	kubeletConfigClient, err := loadKubeletConfigClient()
	if err != nil {
		return err
	}

	containerd := &worker.ContainerD{}
	kubelet := &worker.Kubelet{
		KubeletConfigClient: kubeletConfigClient,
		Profile:             profile,
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
