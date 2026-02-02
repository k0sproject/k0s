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
		Use:   "install",
		Short: "Install k0s on a brand-new system. Must be run as root (or with sudo)",
		Long: `Install k0s as a system service that starts on boot. Must be run as root (or with sudo).

Fails if the service is already installed, unless --force is given. Note that
--force only replaces the service definition. It doesn't stop or restart an
already running service, so the changes take effect once the service gets
restarted. Combine --force with --start to restart it right away.`,
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
	pflags.BoolVar(&installFlags.force, "force", false, "Replace the service definition if the service is already installed")
	pflags.StringArrayVarP(&installFlags.envVars, "env", "e", nil, "Set environment variables (<name>=<value> or just <name>)")
	pflags.BoolVar(&installFlags.start, "start", false, "Start the service immediately after installation (restarts a running service if combined with --force)")

	cmd.AddCommand(installWorkerCmd(&installFlags))
	addPlatformSpecificCommands(cmd, &installFlags)

	return cmd
}
