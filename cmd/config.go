package cmd

import (
	"fmt"
	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

func ConfigCommand() *cli.Command {
	return &cli.Command{
		Name:   "default-config",
		Usage:  "Output the default MKE configuration yaml to stdout",
		Action: buildConfig,
	}
}

func buildConfig(ctx *cli.Context) error {
	conf, _ := yaml.Marshal(v1beta1.DefaultClusterConfig())
	fmt.Print(string(conf))
	return nil
}
