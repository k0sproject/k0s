// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type installFlags struct {
	force   bool
	envVars []string
	start   bool
}

func NewInstallCmd() *cobra.Command {
	var (
		debugFlags   internal.DebugFlags
		installFlags installFlags
	)

	cmd := &cobra.Command{
		Use:              "install",
		Short:            "Install k0s on a brand-new system. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE:             func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	pflags := cmd.PersistentFlags()
	debugFlags.AddToFlagSet(pflags)
	config.GetPersistentFlagSet().VisitAll(func(f *pflag.Flag) {
		f.Hidden = true
		f.Deprecated = "it has no effect and will be removed in a future release"
		pflags.AddFlag(f)
	})
	pflags.BoolVar(&installFlags.force, "force", false, "Force init script creation")
	pflags.StringArrayVarP(&installFlags.envVars, "env", "e", nil, "Set environment variables (<name>=<value> or just <name>)")
	pflags.BoolVar(&installFlags.start, "start", false, "Start the service immediately after installation")

	cmd.AddCommand(installWorkerCmd(&installFlags))
	addPlatformSpecificCommands(cmd, &installFlags)

	return cmd
}
