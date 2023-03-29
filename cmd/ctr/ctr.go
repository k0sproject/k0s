/*
Copyright 2022 k0s authors

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

package ctr

import (
	"os"
	"path"

	"github.com/containerd/containerd/cmd/ctr/app"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/spf13/cobra"
	"github.com/urfave/cli"
)

func NewCtrCommand() *cobra.Command {
	containerdCtr := app.New()
	setDefaultValues(containerdCtr.Flags)

	cmd := &cobra.Command{
		Use:                containerdCtr.Name,
		Short:              "containerd CLI",
		Long:               containerdCtr.Description,
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			args := extractCtrCommand(os.Args)
			return containerdCtr.Run(args)
		},
	}

	return cmd
}

func setDefaultValues(flags []cli.Flag) {
	for i, flag := range flags {
		if f, ok := flag.(cli.StringFlag); ok {
			if f.Name == "address, a" {
				f.Value = path.Join(config.GetCmdOpts().K0sVars.RunDir, "containerd.sock")
				flags[i] = f
			} else if f.Name == "namespace, n" {
				f.Value = "k8s.io"
				flags[i] = f
			}
		}
	}
}

func extractCtrCommand(osArgs []string) []string {
	var args []string
	ctrArgFound := false
	for _, arg := range osArgs {
		if arg == "ctr" {
			ctrArgFound = true
		}
		if ctrArgFound {
			args = append(args, arg)
		}
	}
	return args
}
