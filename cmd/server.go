package cmd

import (
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/Mirantis/mke/pkg/applier"
	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/constant"
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
	err := assets.Stage(path.Join(constant.DataDir))
	if err != nil {
		return err
	}

	spec := config.DefaultClusterSpec()

	components := make(map[string]component.Component)

	certs := component.Certificates{}

	if err := certs.Run(); err != nil {
		return err
	}

	// Block signal til we started up all components
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	components["kine"] = &component.Kine{
		Config: spec.Storage.Kine,
	}
	components["kine"].Run()

	components["kube-apiserver"] = &component.ApiServer{}
	components["kube-apiserver"].Run()

	components["kube-scheduler"] = &component.Scheduler{}
	components["kube-scheduler"].Run()

	components["kube-ccm"] = &component.ControllerManager{}
	components["kube-ccm"].Run()

	// TODO Figure out proper way to wait for api to be alive
	time.Sleep(20 * time.Second)

	components["bundle-manager"] = &applier.Manager{}
	components["bundle-manager"].Run()

	// Wait for mke process termination
	<-c

	components["kube-ccm"].Stop()
	components["kube-scheduler"].Stop()
	components["kube-apiserver"].Stop()
	components["kine"].Stop()

	return nil

}
