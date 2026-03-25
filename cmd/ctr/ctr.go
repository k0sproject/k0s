// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package ctr

import (
	"os"

	"github.com/containerd/containerd/v2/cmd/ctr/app"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/spf13/cobra"
	"github.com/urfave/cli/v2"
)

const pathEnv = "PATH"

func NewCtrCommand() *cobra.Command {
	containerdCtr := app.New()

	cmd := &cobra.Command{
		Use:                containerdCtr.Name,
		Short:              "containerd CLI",
		Long:               containerdCtr.Description,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			setDefaultValues(opts.K0sVars.RunDir, containerdCtr.Flags)
			args := extractCtrCommand(os.Args)
			newPath := dir.PathListJoin(opts.K0sVars.BinDir, os.Getenv(pathEnv))
			os.Setenv(pathEnv, newPath)
			return containerdCtr.Run(args)
		},
	}

	return cmd
}

func setDefaultValues(runDir string, flags []cli.Flag) {
	for i, flag := range flags {
		if f, ok := flag.(*cli.StringFlag); ok {
			switch f.Name {
			case "address":
				f.Value = containerd.Address(runDir)
				flags[i] = f
			case "namespace":
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
