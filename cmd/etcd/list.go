// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"encoding/json"
	"fmt"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/spf13/cobra"
)

func etcdListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member-list",
		Short: "List etcd cluster members (JSON encoded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			etcdClient, err := etcd.NewClient(opts.K0sVars.CertRootDir, opts.K0sVars.EtcdCertDir, nodeConfig.Spec.Storage.Etcd)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %w", err)
			}
			defer etcdClient.Close()
			members, err := etcdClient.ListMembers(ctx)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %w", err)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"members": members})
		},
	}

	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())

	return cmd
}
