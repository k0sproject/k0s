package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mirantis/mke/pkg/applier"
	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/component/server"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

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
			&cli.StringFlag{
				Name:  "join-address",
				Usage: "The address of a pre-existing controlplane node to join into to create HA controlplane",
			},
		},
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
	}
	components := make(map[string]component.Component)

	joinAddress := ctx.String("join-address")
	if joinAddress != "" {
		token := ctx.Args().First()
		if token == "" {
			return fmt.Errorf("need to give the controlplane join token as first argument")
		}
		components["ca-syncer"] = &server.CASyncer{
			JoinAddress: joinAddress,
			Token:       token,
		}

		err = components["ca-syncer"].Init()
		err = components["ca-syncer"].Run()
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
		components["storage"] = &server.Kine{
			Config: clusterConfig.Spec.Storage.Kine,
		}
	case "etcd":
		components["storage"] = &server.Etcd{}
	default:
		return errors.New(fmt.Sprintf("Invalid storage type: %s", clusterConfig.Spec.Storage.Type))
	}
	logrus.Infof("Using storage backend %s", clusterConfig.Spec.Storage.Type)

	components["kube-apiserver"] = &server.ApiServer{
		ClusterConfig: clusterConfig,
	}
	components["kube-scheduler"] = &server.Scheduler{}
	components["kube-ccm"] = &server.ControllerManager{
		ClusterConfig: clusterConfig,
	}
	components["bundle-manager"] = &applier.Manager{}
	components["mke-controlapi"] = &server.MkeControlApi{}

	// extract needed components
	for _, comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	certs := server.NewCertificates(clusterConfig.Spec)
	if err := certs.Run(); err != nil {
		return err
	}

	// Block signal til we started up all components
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Components started one-by-one as there's specific order we want
	components["storage"].Run()
	components["kube-apiserver"].Run()
	components["kube-scheduler"].Run()
	components["kube-ccm"].Run()
	components["bundle-manager"].Run()
	components["mke-controlapi"].Run()

	// in-cluster component reconcilers
	reconcilers := createClusterReconcilers(clusterConfig.Spec)

	// Start all reconcilers
	for _, reconciler := range reconcilers {
		reconciler.Run()
	}

	// Wait for mke process termination
	<-c

	// Stop all reconcilers first
	for _, reconciler := range reconcilers {
		reconciler.Stop()
	}

	// There's specific order we want to shutdown things
	components["mke-controlapi"].Stop()
	components["bundle-manager"].Stop()
	components["kube-ccm"].Stop()
	components["kube-scheduler"].Stop()
	components["kube-apiserver"].Stop()
	components["storage"].Stop()

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

	calico, err := server.NewCalico()
	if err != nil {
		logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
	} else {
		reconcilers["calico"] = calico
	}

	return reconcilers
}
