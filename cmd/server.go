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

	components["containerd"] = component.ContainerD{}
	components["containerd"].Run()

	components["kubelet"] = component.Kubelet{}
	components["kubelet"].Run()

	components["etcd"] = component.Etcd{}
	components["etcd"].Run()

	// Wait for mke process termination
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Stop stuff does not really work yet
	// for _, comp := range components {
	// 	comp.Stop()
	// }

	return nil

}
