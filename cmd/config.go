/*
Copyright 2020 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
