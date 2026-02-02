// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package stop

import (
	"errors"
	"os"
	"runtime"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/install"
	"github.com/k0sproject/k0s/pkg/sysservice"

	"github.com/spf13/cobra"
)

func NewStopCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:              "stop",
		Short:            "Stop the k0s service configured on this host. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "windows" && os.Geteuid() != 0 {
				return errors.New("this command must be run as root")
			}

			ctx := cmd.Context()
			svc, err := install.InstalledService(ctx)
			if err != nil {
				return err
			}
			status, err := svc.Status(ctx)
			if err != nil {
				return err
			}
			if status == sysservice.StatusStopped {
				return errors.New("already stopped")
			}
			return svc.Stop(ctx)
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	return cmd
}
