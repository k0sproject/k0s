// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func etcdUpdateCmd() *cobra.Command {
	var peerAddressArg string
	var memberName string

	cmd := &cobra.Command{
		Use:   "member-update",
		Short: "Update specific member of the cluster",
		// accept peer address list as the first flag
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, peerAddr []string) error {
			for _, peer := range peerAddr {
				if !govalidator.IsIP(peer) && !govalidator.IsDNSName(peer) {
					return fmt.Errorf("%q neither an IP address nor a DNS name", peer)
				}
			}

			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			peerAddress := cmp.Or(peerAddressArg, nodeConfig.Spec.Storage.Etcd.PeerAddress)
			if memberName == "" && peerAddress == "" {
				return errors.New("can't update member: no member name or peer address specified")
			}

			etcdClient, err := etcd.NewClient(opts.K0sVars.CertRootDir, opts.K0sVars.EtcdCertDir, nodeConfig.Spec.Storage.Etcd)
			if err != nil {
				return fmt.Errorf("can't connect to the etcd: %w", err)
			}

			var peerID uint64
			if memberName != "" {
				peerID, err = etcdClient.GetPeerIDByName(ctx, memberName)
				if err != nil {
					logrus.WithField("memberName", memberName).Errorf("Failed to get peer ID")
					return err
				}
			} else if peerAddress != "" {
				peerURL := (&url.URL{Scheme: "https", Host: net.JoinHostPort(peerAddress, "2380")}).String()
				peerID, err = etcdClient.GetPeerIDByAddress(ctx, peerURL)
				if err != nil {
					logrus.WithField("peerURL", peerURL).Errorf("Failed to get peer ID")
					return err
				}
			}

			if err := etcdClient.UpdateMember(ctx, peerID, peerAddr); err != nil {
				logrus.
					WithField("peerID", strconv.FormatUint(peerID, 16)).
					Errorf("Failed to update cluster member")
				return err
			}

			logrus.
				WithField("peerID", strconv.FormatUint(peerID, 16)).
				Info("Successfully updated")
			return nil
		},
	}

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlag(&pflag.Flag{
		Name:  "peer-address",
		Usage: "etcd peer address to update (default <this node's peer address>)",
		Value: (*ipOrDNSName)(&peerAddressArg),
	})
	flags.AddFlag(&pflag.Flag{
		Name:  "member-name",
		Usage: "etcd member name to update",
		Value: (*ipOrDNSName)(&memberName),
	})

	return cmd
}
