package cmd

import (
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/Mirantis/mke/pkg/assets"
	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/urfave/cli/v2"
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

	components := make(map[string]component.Component)

	certs := component.Certificates{}

	if err := certs.Run(); err != nil {
		return err
	}

	// Block signal til we started up all components
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	components["kine"] = &component.Kine{}
	components["kine"].Run()

	components["kube-apiserver"] = &component.ApiServer{}
	components["kube-apiserver"].Run()

	components["kube-scheduler"] = &component.Scheduler{}
	components["kube-scheduler"].Run()

	components["kube-ccm"] = &component.ControllerManager{}
	components["kube-ccm"].Run()

	// Wait for mke process termination
	<-c

	components["kube-ccm"].Stop()
	components["kube-scheduler"].Stop()
	components["kube-apiserver"].Stop()
	components["kine"].Stop()

	return nil

}
