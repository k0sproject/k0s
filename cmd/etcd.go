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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "mke.yaml",
			},
		},
		Subcommands: []*cli.Command{
			LeaveCommand(),
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
			clusterConfig := ConfigFromYaml(c)
			// if there would be any more commands for etcd management
			// it's better to move that check to the Before hook
			if clusterConfig.Spec.Storage.Type != v1beta1.EtcdStorageType {
				return fmt.Errorf("wrong storage type: %s", clusterConfig.Spec.Storage.Type)
			}
			peerAddress := c.String("peer-address")
			if peerAddress == "" {
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
