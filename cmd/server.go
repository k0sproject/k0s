package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Mirantis/mke/pkg/applier"
	"github.com/Mirantis/mke/pkg/component"
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
	spec := config.DefaultClusterSpec()

	components := make(map[string]component.Component)

	components["kine"] = &component.Kine{
		Config: spec.Storage.Kine,
	}
	components["kube-apiserver"] = &component.ApiServer{}
	components["kube-scheduler"] = &component.Scheduler{}
	components["kube-ccm"] = &component.ControllerManager{}
	components["bundle-manager"] = &applier.Manager{}

	// extract needed components
	for _,comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	certs := component.Certificates{}

	if err := certs.Run(); err != nil {
		return err
	}

	// Block signal til we started up all components
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	components["kine"].Run()
	components["kube-apiserver"].Run()
	components["kube-scheduler"].Run()
	components["kube-ccm"].Run()
	components["bundle-manager"].Run()

	// Wait for mke process termination
	<-c

	// There's specific order we want to shutdown things
	components["bundle-manager"].Stop()
	components["kube-ccm"].Stop()
	components["kube-scheduler"].Stop()
	components["kube-apiserver"].Stop()
	components["kine"].Stop()

	return nil
}
