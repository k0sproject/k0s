package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Mirantis/mke/pkg/applier"
	"github.com/Mirantis/mke/pkg/component"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// ServerCommand ...
func ServerCommand() *cli.Command {
	return &cli.Command{
		Name:            "server",
		Usage:           "Run server",
		Action:          startServer,
		SkipFlagParsing: true,
	}
}

func startServer(ctx *cli.Context) error {
	clusterConfig, err := config.FromYaml("mke.yaml")
	if err != nil {
		logrus.Errorf("Failed to read cluster config: %s", err.Error())
		logrus.Error("THINGS MIGHT NOT WORK PROPERLY AS WE'RE GONNA USE DEFAULTS")
		clusterConfig = &config.ClusterConfig{
			Spec: config.DefaultClusterSpec(),
		}
	}

	logrus.Infof("using public address: %s", clusterConfig.Spec.API.Address)
	logrus.Infof("using sans: %s", clusterConfig.Spec.API.SANs)
	dnsAddress, err := clusterConfig.Spec.Network.DNSAddress()
	if err != nil {
		return err
	}
	logrus.Infof("DNS address: %s", dnsAddress)

	// os.Exit(42)

	components := make(map[string]component.Component)

	components["kine"] = &component.Kine{
		Config: clusterConfig.Spec.Storage.Kine,
	}
	components["kube-apiserver"] = &component.ApiServer{
		ClusterConfig: clusterConfig,
	}
	components["kube-scheduler"] = &component.Scheduler{}
	components["kube-ccm"] = &component.ControllerManager{
		ClusterConfig: clusterConfig,
	}
	components["bundle-manager"] = &applier.Manager{}

	// extract needed components
	for _, comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	certs := component.NewCertificates(clusterConfig.Spec)
	if err := certs.Run(); err != nil {
		return err
	}

	// Block signal til we started up all components
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Components started one-by-one as there's specific order we want
	components["kine"] = &component.Kine{
		Config: clusterConfig.Spec.Storage.Kine,
	}
	components["kine"].Run()
	components["kube-apiserver"].Run()
	components["kube-scheduler"].Run()
	components["kube-ccm"].Run()
	components["bundle-manager"].Run()

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
	components["bundle-manager"].Stop()
	components["kube-ccm"].Stop()
	components["kube-scheduler"].Stop()
	components["kube-apiserver"].Stop()
	components["kine"].Stop()

	return nil
}

func createClusterReconcilers(clusterSpec *config.ClusterSpec) map[string]component.Component {
	reconcilers := make(map[string]component.Component)

	proxy, err := component.NewKubeProxy(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize kube-proxy reconciler: %s", err.Error())
	} else {
		reconcilers["kube-proxy"] = proxy
	}

	coreDNS, err := component.NewCoreDNS(clusterSpec)
	if err != nil {
		logrus.Warnf("failed to initialize CoreDNS reconciler: %s", err.Error())
	} else {
		reconcilers["coredns"] = coreDNS
	}

	calico, err := component.NewCalico()
	if err != nil {
		logrus.Warnf("failed to initialize calico reconciler: %s", err.Error())
	} else {
		reconcilers["calico"] = calico
	}

	return reconcilers
}
