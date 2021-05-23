package ctr

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/cmd/ctr/app"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/urfave/cli"
)

func NewCtrCommand() *cobra.Command {
	originalCtr := app.New()

	internalArgs := []string{"ctr"}
	ctrCommand := &cobra.Command{
		Use:   originalCtr.Name,
		Short: "containerd CLI",
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
			flags := cmdFlagsToArgs(cmd)
			internalArgs = append(internalArgs, flags...)
			internalArgs = append(internalArgs, args...)
			return originalCmd.Run(internalArgs)
		},
	}
	parentCmd.AddCommand(command)

	configureFlags(command, cmd.Flags)

	for _, subCmd := range cmd.Subcommands {
		configureCommand(originalCmd, command, subCmd, internalArgs)
	}
}

func configureFlags(cmd *cobra.Command, flags []cli.Flag) {
	for _, f := range flags {
		switch flag := f.(type) {
		case cli.BoolFlag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().Bool(name, false, flag.Usage)
			} else {
				cmd.Flags().BoolP(name, shorthand, false, flag.Usage)
			}
		case cli.IntFlag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().Int(name, flag.Value, flag.Usage)
			} else {
				cmd.Flags().IntP(name, shorthand, flag.Value, flag.Usage)
			}
		case cli.Uint64Flag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().Uint64(name, flag.Value, flag.Usage)
			} else {
				cmd.Flags().Uint64P(name, shorthand, flag.Value, flag.Usage)
			}
		case cli.Float64Flag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().Float64(name, flag.Value, flag.Usage)
			} else {
				cmd.Flags().Float64P(name, shorthand, flag.Value, flag.Usage)
			}
		case cli.StringFlag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().String(name, flag.Value, flag.Usage)
			} else {
				cmd.Flags().StringP(name, shorthand, flag.Value, flag.Usage)
			}
		case cli.StringSliceFlag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().StringSlice(name, getValueSafely(flag.Value), flag.Usage)
			} else {
				cmd.Flags().StringSliceP(name, shorthand, getValueSafely(flag.Value), flag.Usage)
			}
		case cli.DurationFlag:
			if name, shorthand := splitNames(flag.Name); shorthand == "" {
				cmd.Flags().Duration(name, flag.Value, flag.Usage)
			} else {
				cmd.Flags().DurationP(name, shorthand, flag.Value, flag.Usage)
			}
		}
	}
}

func splitNames(name string) (string, string) {
	names := strings.SplitN(name, ",", 2)
	for i, name := range names {
		names[i] = strings.TrimSpace(name)
	}

	if len(names) == 2 {
		return names[0], names[1]
	} else {
		return names[0], ""
	}
}

func getValueSafely(s *cli.StringSlice) []string {
	if s != nil {
		return s.Value()
	} else {
		return []string{}
	}
}

func cmdFlagsToArgs(cmd *cobra.Command) []string {
	var flagsAndVals []string
	// Use visitor to collect all flags and vals into slice
	cmd.Flags().Visit(func(f *pflag.Flag) {
		val := f.Value.String()
		switch f.Value.Type() {
		case "stringSlice", "stringToString":
			flagsAndVals = append(flagsAndVals, fmt.Sprintf(`--%s="%s"`, f.Name, strings.Trim(val, "[]")))
		default:
			if f.Name == "data-dir" || f.Name == "token-file" || f.Name == "config-file" {
				val, _ = filepath.Abs(val)
			}
			flagsAndVals = append(flagsAndVals, fmt.Sprintf("--%s=%s", f.Name, val))
		}
	})
	return flagsAndVals
}
