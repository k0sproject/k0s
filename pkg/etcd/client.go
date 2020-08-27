package etcd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/constant"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

type Client struct {
	client *clientv3.Client
}

func NewClient() (*Client, error) {
	client := &Client{}
	tlsInfo := transport.TLSInfo{
		CertFile:      filepath.Join(constant.CertRoot, "apiserver-etcd-client.crt"),
		KeyFile:       filepath.Join(constant.CertRoot, "apiserver-etcd-client.key"),
		TrustedCAFile: filepath.Join(constant.CertRoot, "etcd", "ca.crt"),
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	cli, _ := clientv3.New(clientv3.Config{
		Endpoints: []string{"https://127.0.0.1:2379"},
		TLS:       tlsConfig,
	})

	client.client = cli

	return client, nil
}

func (c *Client) ListMembers() (map[string]string, error) {
	memberList := make(map[string]string)
	members, err := c.client.MemberList(context.TODO())
	if err != nil {
		return nil, err
	}
	for _, m := range members.Members {
		logrus.Infof("peer: %s, peerAddresses: %v", m.Name, m.PeerURLs)
		memberList[m.Name] = m.PeerURLs[0]
	}

	return memberList, nil
}

func (c *Client) AddMember(name, peerAddress string) ([]string, error) {

	addResp, err := c.client.MemberAdd(context.TODO(), []string{peerAddress})
	if err != nil {
		// TODO we should try to detect possible double add for a peer
		// Not sure though if we can return correct initial-cluster as the order
		// is important for the peers :/
		return nil, err
	}

	newID := addResp.Member.ID

	memberList := []string{}
	for _, m := range addResp.Members {
		memberName := m.Name
		if m.ID == newID {
			memberName = name
		}
		memberList = append(memberList, fmt.Sprintf("%s=%s", memberName, m.PeerURLs[0]))
	}

	return memberList, nil
}

func (c *Client) Close() {
	c.client.Close()
}
