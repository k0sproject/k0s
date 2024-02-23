/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/asaskevich/govalidator"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func etcdLeaveCmd() *cobra.Command {
	var peerAddressArg string

	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave the etcd cluster, or remove a specific peer",
		Args:  cobra.NoArgs, // accept peer address via flag, not via arg
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}
			nodeConfig, err := opts.K0sVars.NodeConfig()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			peerAddress := nodeConfig.Spec.Storage.Etcd.PeerAddress
			if peerAddressArg == "" {
				if peerAddress == "" {
					return fmt.Errorf("can't leave etcd cluster: this node doesn't have an etcd peer address, check the k0s configuration or use --peer-address")
				}
			} else {
				peerAddress = peerAddressArg
			}

			peerURL := (&url.URL{Scheme: "https", Host: net.JoinHostPort(peerAddress, "2380")}).String()
			etcdClient, err := etcd.NewClient(opts.K0sVars.CertRootDir, opts.K0sVars.EtcdCertDir, nodeConfig.Spec.Storage.Etcd)
			if err != nil {
				return fmt.Errorf("can't connect to the etcd: %w", err)
			}

			peerID, err := etcdClient.GetPeerIDByAddress(ctx, peerURL)
			if err != nil {
				logrus.WithField("peerURL", peerURL).Errorf("Failed to get peer name")
				return err
			}

			if err := etcdClient.DeleteMember(ctx, peerID); err != nil {
				logrus.
					WithField("peerURL", peerURL).
					WithField("peerID", fmt.Sprintf("%x", peerID)).
					Errorf("Failed to delete node from cluster")
				return err
			}

			logrus.
				WithField("peerID", fmt.Sprintf("%x", peerID)).
				Info("Successfully deleted")
			return nil
		},
	}

	cmd.Flags().AddFlag(&pflag.Flag{
		Name:  "peer-address",
		Usage: "etcd peer address to remove (default <this node's peer address>)",
		Value: (*ipOrDNSName)(&peerAddressArg),
	})

	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}

type ipOrDNSName string

func (i *ipOrDNSName) Type() string   { return "ip-or-dns-name" }
func (i *ipOrDNSName) String() string { return string(*i) }

func (i *ipOrDNSName) Set(value string) error {
	if !govalidator.IsIP(value) && !govalidator.IsDNSName(value) {
		return errors.New("neither an IP address nor a DNS name")
	}

	*i = ipOrDNSName(value)
	return nil
}
