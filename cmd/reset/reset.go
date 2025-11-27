//go:build linux

// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"errors"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/cleanup"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type command config.CLIOptions

func NewResetCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:              "reset",
		Short:            "Uninstall k0s. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			c := (*command)(opts)
			return c.reset(debugFlags.IsDebug())
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetCriSocketFlag())
	flags.AddFlagSet(config.FileInputFlag())
	flags.String("kubelet-root-dir", "", "Kubelet root directory for k0s")
	flags.String("containerd-root-dir", "", "Containerd root directory for k0s")

	return cmd
}

func (c *command) reset(debug bool) error {
	if os.Geteuid() != 0 {
		return errors.New("this command must be run as root")
	}

	k0sStatus, _ := status.GetStatusInfo(c.K0sVars.StatusSocketPath)
	if k0sStatus != nil && k0sStatus.Pid != 0 {
		return errors.New("k0s seems to be running, please stop k0s before reset")
	}

	nodeCfg, err := c.K0sVars.NodeConfig()
	if err != nil {
		return err
	}
	if nodeCfg.Spec.Storage.Kine != nil && nodeCfg.Spec.Storage.Kine.DataSource != "" {
		logrus.Warn("Kine dataSource is configured. k0s will not reset the data source if it points to an external database. If you plan to continue using the data source, you should reset it to avoid conflicts.")
	}

	// Get Cleanup Config
	cfg, err := cleanup.NewConfig(debug, c.K0sVars, nodeCfg.Spec.Install.SystemUsers, c.CriSocket)
	if err != nil {
		return fmt.Errorf("failed to configure cleanup: %w", err)
	}

	err = cfg.Cleanup()
	logrus.Info("k0s cleanup operations done.")
	logrus.Warn("To ensure a full reset, a node reboot is recommended.")

	return err
}
