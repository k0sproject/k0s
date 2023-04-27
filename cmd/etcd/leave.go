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
	"fmt"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/etcd"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func etcdLeaveCmd() *cobra.Command {
	var etcdPeerAddress string

	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Sign off a given etc node from etcd cluster",
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
			if etcdPeerAddress == "" {
				etcdPeerAddress = nodeConfig.Spec.Storage.Etcd.PeerAddress
			}
			if etcdPeerAddress == "" {
				return fmt.Errorf("can't leave etcd cluster: peer address is empty, check the config file or use cli argument")
			}

			peerURL := fmt.Sprintf("https://%s:2380", etcdPeerAddress)
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
					WithField("peerID", peerID).
					Errorf("Failed to delete node from cluster")
				return err
			}

			logrus.
				WithField("peerID", peerID).
				Info("Successfully deleted")
			return nil
		},
	}

	cmd.Flags().StringVar(&etcdPeerAddress, "peer-address", "", "etcd peer address")
	cmd.PersistentFlags().AddFlagSet(config.GetPersistentFlagSet())
	return cmd
}
