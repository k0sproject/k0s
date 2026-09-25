// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"errors"
	"fmt"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewEtcdCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:   "etcd",
		Short: "Manage etcd cluster",
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			debugFlags.Run(cmd, args)

			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			if nodeConfig.Spec.Storage.Type != v1beta1.EtcdStorageType {
				return fmt.Errorf("wrong storage type: %s", nodeConfig.Spec.Storage.Type)
			}
			if nodeConfig.Spec.Storage.Etcd.IsExternalClusterUsed() {
				return errors.New("command 'k0s etcd' does not support external etcd cluster")
			}
			return nil
		},
		RunE: func(*cobra.Command, []string) error { return pflag.ErrHelp }, // Enforce arg validation
	}

	pflags := cmd.PersistentFlags()
	debugFlags.AddToFlagSet(pflags)
	pflags.AddFlagSet(config.GetPersistentFlagSet())

	cmd.AddCommand(etcdLeaveCmd())
	cmd.AddCommand(etcdUpdateCmd())
	cmd.AddCommand(etcdListCmd())

	return cmd
}
