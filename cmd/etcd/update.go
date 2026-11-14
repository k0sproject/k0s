// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/k0sproject/k0s/internal/k0scontext"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type etcdMemberUpdateClient interface {
	ListMembers(context.Context) ([]etcd.Member, error)
	SetPeerURL(context.Context, uint64, string) error
	Close() error
}

func etcdUpdateCmd() *cobra.Command {
	var memberName, peerAddress string
	cmd := &cobra.Command{
		Use:   "member-update <new-peer-address>",
		Short: "Update an etcd member's peer address",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}
			var address ipOrDNSName
			if err := address.Set(args[0]); err != nil {
				return fmt.Errorf("invalid new peer address %q: %w", args[0], err)
			}
			if cmd.Flags().Changed("member-name") && memberName == "" {
				return errors.New("member name must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			if memberName == "" && peerAddress == "" {
				peerAddress = nodeConfig.Spec.Storage.Etcd.PeerAddress
				if peerAddress == "" {
					return errors.New("can't update etcd member: specify --member-name or --peer-address, or configure this node's etcd peer address")
				}
			}
			peerURL := func(address string) string {
				return (&url.URL{Scheme: "https", Host: net.JoinHostPort(address, "2380")}).String()
			}
			ctx := cmd.Context()
			client := k0scontext.Value[etcdMemberUpdateClient](ctx)
			if client == nil {
				client, err = etcd.NewClient(opts.K0sVars.CertRootDir, opts.K0sVars.EtcdCertDir, nodeConfig.Spec.Storage.Etcd)
				if err != nil {
					return fmt.Errorf("can't connect to etcd: %w", err)
				}
			}
			defer client.Close()
			members, err := client.ListMembers(ctx)
			if err != nil {
				return fmt.Errorf("can't find etcd member: %w", err)
			}
			var matches []etcd.Member
			for _, member := range members {
				if (memberName != "" && member.Name == memberName) || (memberName == "" && member.PeerURL == peerURL(peerAddress)) {
					matches = append(matches, member)
				}
			}
			if len(matches) != 1 {
				return fmt.Errorf("expected exactly one etcd member for name %q or peer address %q, found %d", memberName, peerAddress, len(matches))
			}
			member := matches[0]
			if err := client.SetPeerURL(ctx, member.ID, peerURL(args[0])); err != nil {
				return fmt.Errorf("can't update etcd member %x: %w", member.ID, err)
			}
			logrus.WithField("peerID", strconv.FormatUint(member.ID, 16)).Info("Successfully updated")
			return nil
		},
	}
	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.StringVar(&memberName, "member-name", "", "etcd member name to update")
	flags.AddFlag(&pflag.Flag{
		Name:  "peer-address",
		Usage: "current etcd peer address (default <this node's peer address>)",
		Value: (*ipOrDNSName)(&peerAddress),
	})
	cmd.MarkFlagsMutuallyExclusive("member-name", "peer-address")
	return cmd
}
