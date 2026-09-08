// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package uninstall

import (
	"context"
	"errors"
	"fmt"

	"github.com/k0sproject/k0s/pkg/cleanup"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
)

// Options encapsulates the data required to perform a node reset.
type Options struct {
	Vars      *config.CfgVars
	CriSocket string
	Debug     bool
}

// Run performs the reset orchestration common to all supported operating systems.
func Run(ctx context.Context, opts Options) error {
	if opts.Vars == nil {
		return errors.New("k0s vars must not be nil")
	}

	if err := ensurePrivileges(); err != nil {
		return err
	}

	if locked, err := config.RuntimeConfigLocked(opts.Vars.RuntimeConfigPath); err != nil {
		return err
	} else if locked {
		return config.ErrK0sStillRunning
	}

	nodeCfg, err := opts.Vars.NodeConfig()
	if err != nil {
		return err
	}
	if nodeCfg.Spec.Storage.Kine != nil && nodeCfg.Spec.Storage.Kine.DataSource != "" {
		logrus.Warn("Kine dataSource is configured. k0s will not reset the data source if it points to an external database. If you plan to continue using the data source, you should reset it to avoid conflicts.")
	}

	cfg, err := cleanup.NewConfig(opts.Debug, opts.Vars, nodeCfg.Spec.Install.SystemUsers, opts.CriSocket)
	if err != nil {
		return fmt.Errorf("failed to configure cleanup: %w", err)
	}

	err = cfg.Cleanup(ctx)
	logrus.Info("k0s cleanup operations done.")
	logrus.Warn("To ensure a full reset, a node reboot is recommended.")

	return err
}
