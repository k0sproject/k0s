package main

import (
	"log"
	"os"

	"github.com/Mirantis/mke/cmd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Version gets overridden at build time using -X main.Version=$VERSION
var (
	Version = "dev"
)

func init() {

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)
}

func main() {
	app := &cli.App{
		Name:    "mke",
		Version: Version,
		Usage:   "Mirantis Kubernetes Engine",
		Commands: []*cli.Command{
			cmd.ServerCommand(),
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Debug logging",
				Aliases: []string{"d"},
				EnvVars: []string{"DEBUG"},
			},
		},
		Before: func(ctx *cli.Context) error {
			if ctx.Bool("debug") {
				logrus.SetLevel(logrus.DebugLevel)
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
