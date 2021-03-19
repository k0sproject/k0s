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
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/etcd"
)

func init() {
	etcdLeaveCmd.Flags().StringVar(&etcdPeerAddress, "peer-address", "", "etcd peer address")

	etcdCmd.AddCommand(etcdLeaveCmd)
	etcdCmd.AddCommand(etcdListCmd)
	etcdCmd.AddCommand(etcdHealthCmd)

	addPersistentFlags(etcdCmd)
}

var (
	etcdCmd = &cobra.Command{
		Use:   "etcd",
		Short: "Manage etcd cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			clusterConfig, err := ConfigFromYaml(cfgFile)
			if err != nil {
				return fmt.Errorf("can't read cluster config file")
			}
			if clusterConfig.Spec.Storage.Type != v1beta1.EtcdStorageType {
				return fmt.Errorf("wrong storage type: %s", clusterConfig.Spec.Storage.Type)
			}
			return nil
		},
	}
)

var (
	etcdPeerAddress string

	// etcdLeaveCmd force node to leave etcd cluster
	etcdLeaveCmd = &cobra.Command{
		Use:   "leave",
		Short: "Sign off a given etc node from etcd cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if etcdPeerAddress == "" {
				clusterConfig, err := ConfigFromYaml(cfgFile)
				if err != nil {
					return fmt.Errorf("can't read cluster config file")
				}
				etcdPeerAddress = clusterConfig.Spec.Storage.Etcd.PeerAddress
			}
			if etcdPeerAddress == "" {
				return fmt.Errorf("can't leave etcd cluster: peer address is empty, check the config file or use cli argument")
			}

			peerURL := fmt.Sprintf("https://%s:2380", etcdPeerAddress)
			etcdClient, err := etcd.NewClient(k0sVars.CertRootDir, k0sVars.EtcdCertDir)
			if err != nil {
				return fmt.Errorf("can't connect to the etcd: %v", err)
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
)

var (
	// etcdListCmd returns members of the etcd cluster
	etcdListCmd = &cobra.Command{
		Use:   "member-list",
		Short: "Returns etcd cluster members list",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			etcdClient, err := etcd.NewClient(k0sVars.CertRootDir, k0sVars.EtcdCertDir)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %v", err)
			}
			members, err := etcdClient.ListMembers(ctx)
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %v", err)
			}
			l := logrus.New()
			l.SetFormatter(&logrus.JSONFormatter{})

			l.WithField("members", members).
				Info("done")
			return nil
		},
	}
)

var (
	etcdHealthCmd = &cobra.Command{
		Use:   "health",
		Short: "Returns etcd cluster members health status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			etcdClient, err := etcd.NewClient(k0sVars.CertRootDir, k0sVars.EtcdCertDir)
			if err != nil {
				return fmt.Errorf("can't create etcd client: %v", err)
			}

			context, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			err = etcdClient.Health(context)
			if err != nil {
				return err
			}

			fmt.Println("etcd healthy")

			return nil
		},
	}
)
