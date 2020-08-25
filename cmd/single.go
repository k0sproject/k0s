package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mirantis/mke/pkg/component"
	"github.com/Mirantis/mke/pkg/component/single"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// SingleCommand ...
func SingleCommand() *cli.Command {
	return &cli.Command{
		Name:   "single",
		Usage:  "Run both server and worker as single node cluster",
		Action: startSingle,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "test",
				Usage: "Shut down after bootstrap",
			},
		},
	}
}

func startSingle(ctx *cli.Context) error {
	components := make(map[string]component.Component)

	components["mke-server"] = &single.MkeServer{
		Debug: ctx.Bool("debug"),
	}
	components["mke-worker"] = &single.MkeWorker{
		Debug: ctx.Bool("debug"),
	}

	// extract needed components
	for _, comp := range components {
		if err := comp.Init(); err != nil {
			return err
		}
	}

	// Set up signal handling. Use bufferend channel so we dont miss
	// signals during startup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	components["mke-server"].Run()
	components["mke-worker"].Run()

	// Wait for mke process termination
	if ctx.Bool("test") {
		select {
		case <-c:
		case <-time.After(5 * time.Second):
		}
	} else {
		<-c
	}
	logrus.Info("Shutting down mke single")

	components["mke-worker"].Stop()
	components["mke-server"].Stop()

	return nil

}
