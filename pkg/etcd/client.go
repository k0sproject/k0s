package etcd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/constant"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

// Client is our internal helper to access some of the etcd APIs
type Client struct {
	client *clientv3.Client
}

// NewClient creates new Client
func NewClient() (*Client, error) {
	client := &Client{}
	tlsInfo := transport.TLSInfo{
		CertFile:      filepath.Join(constant.CertRootDir, "apiserver-etcd-client.crt"),
		KeyFile:       filepath.Join(constant.CertRootDir, "apiserver-etcd-client.key"),
		TrustedCAFile: filepath.Join(constant.CertRootDir, "etcd", "ca.crt"),
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

// ListMembers gets a list of current etcd members
func (c *Client) ListMembers(ctx context.Context) (map[string]string, error) {
	memberList := make(map[string]string)
	members, err := c.client.MemberList(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range members.Members {
		memberList[m.Name] = m.PeerURLs[0]
	}

	return memberList, nil
}

// AddMember add new member to etcd cluster
func (c *Client) AddMember(ctx context.Context, name, peerAddress string) ([]string, error) {

	addResp, err := c.client.MemberAdd(ctx, []string{peerAddress})
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

// GetPeerIDByAddress looks up peer id by peer url
func (c *Client) GetPeerIDByAddress(ctx context.Context, peerAddress string) (uint64, error) {
	resp, err := c.client.MemberList(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "etcd member list failed")
	}
	for _, m := range resp.Members {
		for _, peerURL := range m.PeerURLs {
			if peerURL == peerAddress {
				return m.ID, nil
			}
		}
	}
	return 0, errors.Errorf("peer not found: %s", peerAddress)
}

// DeleteMember deletes member by peer name
func (c *Client) DeleteMember(ctx context.Context, peerID uint64) error {
	_, err := c.client.MemberRemove(ctx, peerID)
	return err
}

// Close closes the etcd client
func (c *Client) Close() {
	c.client.Close()
}
