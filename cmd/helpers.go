package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
)

// ConfigFromYaml returns given MKE config or default config
func ConfigFromYaml(ctx *cli.Context) *config.ClusterConfig {
	clusterConfig, err := config.FromYaml(ctx.String("config"))
	if err != nil {
		logrus.Errorf("Failed to read cluster config: %s", err.Error())
		logrus.Error("THINGS MIGHT NOT WORK PROPERLY AS WE'RE GONNA USE DEFAULTS")
		clusterConfig = &config.ClusterConfig{
			Spec: config.DefaultClusterSpec(),
		}
	}
	return clusterConfig
}
