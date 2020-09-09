package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

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
		},
		ArgsUsage: "[join-token]",
	}
}

func startServer(ctx *cli.Context) error {
	clusterConfig, err := config.FromYaml(ctx.String("config"))
	if err != nil {
		logrus.Errorf("Failed to read cluster config: %s", err.Error())
		logrus.Error("THINGS MIGHT NOT WORK PROPERLY AS WE'RE GONNA USE DEFAULTS")
		clusterConfig = &config.ClusterConfig{
			Spec: config.DefaultClusterSpec(),
		}
	} else {
		errors := clusterConfig.Validate()
		if len(errors) > 0 {
			messages := make([]string, len(errors))
			for _, e := range errors {
				messages = append(messages, e.Error())
			}
			return fmt.Errorf("config yaml does not pass validation, following errors found:%s", strings.Join(messages, "\n"))
		}
	}
	componentManager := component.NewManager()
	certificateManager := certificate.Manager{}

	var join = false
	var joinClient *v1beta1.JoinClient
	token := ctx.Args().First()
	if token != "" {
		join = true
		joinClient, err = v1beta1.JoinClientFromToken(token)
		if err != nil {
			return errors.Wrapf(err, "failed to create join client")
		}
		caSyncer := &server.CASyncer{
			JoinClient: joinClient,
		}

		err = caSyncer.Init()
		err = caSyncer.Run()
		if err != nil {
			logrus.Warnf("something failed in CA sync: %s", err.Error())
		}
	}

	logrus.Infof("using public address: %s", clusterConfig.Spec.API.Address)
	logrus.Infof("using sans: %s", clusterConfig.Spec.API.SANs)
	dnsAddress, err := clusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)

	switch clusterConfig.Spec.Storage.Type {
	case "kine", "":
		componentManager.Add(&server.Kine{
			Config: clusterConfig.Spec.Storage.Kine,
		})
	case "etcd":
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

	componentManager.Add(&server.ApiServer{
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
	componentManager.Add(&server.MkeControlApi{})

	// init components
	if err := componentManager.Init(); err != nil {
		return err
	}

	certs := server.Certificates{
		ClusterSpec: clusterConfig.Spec,
		CertManager: certificateManager,
	}
	if err := certs.Run(); err != nil {
		return err
	}

	// Set up signal handling. Use bufferend channel so we dont miss
	// signals during startup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start components
	err = componentManager.Start()
	if err != nil {
		logrus.Errorf("failed to start server components: %s", err)
		c <- syscall.SIGTERM
	}

	// in-cluster component reconcilers
	reconcilers := createClusterReconcilers(clusterConfig.Spec)
	if err == nil {
		// Start all reconcilers
		for _, reconciler := range reconcilers {
			reconciler.Run()
		}
	}

	if err == nil && ctx.Bool("enable-worker") {
		err = enableServerWorker(clusterConfig, componentManager)
		if err != nil {
			logrus.Errorf("failed to start worker components: %s", err)
			componentManager.Stop()
			return err
		}
	}

	// Wait for mke process termination
	<-c
	logrus.Info("Shutting down mke server")

	// Stop all reconcilers first
	for _, reconciler := range reconcilers {
		reconciler.Stop()
	}

	// Stop components
	componentManager.Stop()

	return nil
}

func createClusterReconcilers(clusterSpec *config.ClusterSpec) map[string]component.Component {
	reconcilers := make(map[string]component.Component)

	proxy, err := server.NewKubeProxy(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := server.NewCoreDNS(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	if clusterSpec.Network.Provider == "calico" {
		calico, err := server.NewCalico(clusterSpec)
		if err != nil {
			logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
		} else {
			reconcilers["calico"] = calico
		}
	} else {
		logrus.Warnf("network provider set to custom, mke will not manage it", clusterSpec.Network.Provider)
	}

	metricServer, err := server.NewMetricServer(clusterSpec)
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

func enableServerWorker(clusterConfig *config.ClusterConfig, componentManager *component.Manager) error {
	if !util.FileExists(path.Join(constant.DataDir, "kubelet.conf")) {
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
	}
	containerd.Init()
	kubelet.Init()
	containerd.Run()
	kubelet.Run()

	componentManager.Add(containerd)
	componentManager.Add(kubelet)

	return nil
}
