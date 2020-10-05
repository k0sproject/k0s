package cmd

import (
	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/urfave/cli/v2"
)

// EtcdCommand manages etcd cluster
func EtcdCommand() *cli.Command {
	return &cli.Command{
		Name:  "etcd",
		Usage: "Manage etcd cluster",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "mke.yaml",
			},
		},
		Subcommands: []*cli.Command{
			LeaveCommand(),
		},
	}
}

// LeaveCommand force node to leave etcd cluster
func LeaveCommand() *cli.Command {
	return &cli.Command{
		Name:  "leave",
		Usage: "Sign off a given etc node from etcd cluster",
		Action: func(ctx *cli.Context) error {
			clusterConfig := ConfigFromYaml(ctx)
			clusterConfig.Spec.Storage.Etc
			return nil
		},
	}
}
