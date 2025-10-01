// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package start

import (
	"errors"
	"os"
	"runtime"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:              "start",
		Short:            "Start the k0s service configured on this host. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "windows" && os.Geteuid() != 0 {
				return errors.New("this command must be run as root")
			}
			svc, err := install.InstalledService()
			if err != nil {
				return err
			}
			status, _ := svc.Status()
			if status == service.StatusRunning {
				return errors.New("already running")
			}
			return svc.Start()
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	return cmd
}
