package ctr

import (
	"github.com/containerd/containerd/cmd/ctr/app"
	"github.com/spf13/cobra"
	"github.com/urfave/cli"
)

func NewCtrCommand() *cobra.Command {
	originalCtr := app.New()

	internalArgs := []string{"ctr"}
	ctrCommand := &cobra.Command{
		Use:   originalCtr.Name,
		Short: originalCtr.Usage,
		Long:  originalCtr.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			internalArgs = append(internalArgs, args...)
			return originalCtr.Run(internalArgs)
		},
	}

	for _, command := range originalCtr.Commands {
		configureCommand(originalCtr, ctrCommand, command, internalArgs)
	}

	return ctrCommand
}

func configureCommand(originalCmd *cli.App, parentCmd *cobra.Command, cmd cli.Command, internalArgs []string) {
	internalArgs = append(internalArgs, cmd.Name)

	command := &cobra.Command{
		Use:   cmd.Name,
		Short: cmd.Usage,
		Long:  cmd.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			internalArgs = append(internalArgs, args...)
			return originalCmd.Run(internalArgs)
		},
	}
	parentCmd.AddCommand(command)

	for _, subCmd := range cmd.Subcommands {
		configureCommand(originalCmd, command, subCmd, internalArgs)
	}
}


