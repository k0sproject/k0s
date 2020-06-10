package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"path"

	"github.com/Mirantis/mke/pkg/assets"
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
	kubeletBin := path.Join(constant.DataDir, "bin", "kubelet")
	cmd := exec.Command(kubeletBin, ctx.Args().Slice()...)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", stdoutStderr)
	return nil
}
