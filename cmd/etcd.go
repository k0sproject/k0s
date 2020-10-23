/*
Copyright 2020 Mirantis, Inc.

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
	"fmt"
	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/etcd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// EtcdCommand manages etcd cluster
func EtcdCommand() *cli.Command {
	return &cli.Command{
		Name:  "etcd",
		Usage: "Manage etcd cluster",
		Before: func(c *cli.Context) error {
			clusterConfig := ConfigFromYaml(c)
			if clusterConfig.Spec.Storage.Type != v1beta1.EtcdStorageType {
				return fmt.Errorf("wrong storage type: %s", clusterConfig.Spec.Storage.Type)
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "mke.yaml",
			},
		},
		Subcommands: []*cli.Command{
			LeaveCommand(),
			ListCommand(),
		},
	}
}

// LeaveCommand force node to leave etcd cluster
func LeaveCommand() *cli.Command {
	return &cli.Command{
		Name:  "leave",
		Usage: "Sign off a given etc node from etcd cluster",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "peer-address",
			},
		},
		Action: func(c *cli.Context) error {
			peerAddress := c.String("peer-address")
			if peerAddress == "" {
				clusterConfig := ConfigFromYaml(c)
				peerAddress = clusterConfig.Spec.Storage.Etcd.PeerAddress
			}
			if peerAddress == "" {
				return fmt.Errorf("can't leave etcd cluster: peer address is empty, check the config file or use cli argument")
			}

			peerURL := fmt.Sprintf("https://%s:2380", peerAddress)

			etcdClient, err := etcd.NewClient()

			if err != nil {
				return fmt.Errorf("can't connect to the etcd: %v", err)
			}

			peerID, err := etcdClient.GetPeerIDByAddress(c.Context, peerURL)
			if err != nil {
				logrus.WithField("peerURL", peerURL).Errorf("Failed to get peer name")
				return err
			}

			if err := etcdClient.DeleteMember(c.Context, peerID); err != nil {
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
}

// ListCommand returns members of the etcd cluster
func ListCommand() *cli.Command {
	return &cli.Command{
		Name:  "member-list",
		Usage: "returns etcd cluster members list",
		Action: func(c *cli.Context) error {
			etcdClient, err := etcd.NewClient()
			if err != nil {
				return fmt.Errorf("can't list etcd cluster members: %v", err)
			}
			members, err := etcdClient.ListMembers(c.Context)
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

}
