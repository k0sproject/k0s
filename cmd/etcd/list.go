// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/k0sproject/k0s/pkg/k0scontext"

	"github.com/spf13/cobra"
)

type etcdMemberListClient interface {
	ListMembers(context.Context) ([]etcd.Member, error)
	Close() error
}

func etcdListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member-list",
		Short: "List etcd cluster members (JSON encoded)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			etcdClient := k0scontext.Value[etcdMemberListClient](ctx)
			if etcdClient == nil {
				etcdClient, err = etcd.NewClient(opts.K0sVars.CertRootDir, opts.K0sVars.EtcdCertDir, nodeConfig.Spec.Storage.Etcd)
				if err != nil {
					return fmt.Errorf("can't list etcd cluster members: %w", err)
				}
			}
			defer etcdClient.Close()

			members, err := etcdClient.ListMembers(ctx)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %w", err)
			}
			response := struct {
				Members map[string]string `json:"members"`
			}{
				make(map[string]string, len(members)),
			}
			for _, member := range members {
				response.Members[member.Name] = member.PeerURL
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
		},
	}

	cmd.Flags().AddFlagSet(config.GetPersistentFlagSet())

	return cmd
}
