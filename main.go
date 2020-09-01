package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Mirantis/mke/cmd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

//go:generate go run gen_bindata.go -pkg assets -gofile pkg/assets/zz_generated_offsets.go -prefix embedded-bins/staging/linux/ embedded-bins/staging/linux/bin

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
			cmd.WorkerCommand(),
			cmd.TokenCommand(),
			cmd.SingleCommand(),
			cmd.ApiCommand(),
			versionCommand(),
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

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print version info",
		Action: func(ctx *cli.Context) error {
			fmt.Println(Version)
			return nil
		},
	}
}
