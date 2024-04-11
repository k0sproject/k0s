/*
Copyright 2020 k0s authors

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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Client is our internal helper to access some of the etcd APIs
type Client struct {
	Config  *clientv3.Config
	client  *clientv3.Client
	tlsInfo transport.TLSInfo
}

// NewClient creates new Client
func NewClient(certDir, etcdCertDir string, etcdConf *v1beta1.EtcdConfig) (*Client, error) {
	client := &Client{}

	var tlsConfig *tls.Config
	if etcdConf.IsTLSEnabled() {
		client.tlsInfo = transport.TLSInfo{
			CertFile:      etcdConf.GetCertFilePath(certDir),
			KeyFile:       etcdConf.GetKeyFilePath(certDir),
			TrustedCAFile: etcdConf.GetCaFilePath(etcdCertDir),
		}

		var err error
		tlsConfig, err = client.tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	cfg := clientv3.Config{
		Endpoints: etcdConf.GetEndpoints(),
		TLS:       tlsConfig,
	}
	return NewClientWithConfig(cfg)
}

func NewClientWithConfig(cfg clientv3.Config) (*Client, error) {
	client := &Client{}

	cli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("can't build etcd client: %w", err)
	}
	client.client = cli
	client.Config = &cfg
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

	var memberList []string
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
		return 0, fmt.Errorf("etcd member list failed: %w", err)
	}
	for _, m := range resp.Members {
		for _, peerURL := range m.PeerURLs {
			if peerURL == peerAddress {
				return m.ID, nil
			}
		}
	}
	return 0, fmt.Errorf("peer not found: %s", peerAddress)
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

// Health return err if the etcd peer is not reported as healthy
// ref: https://github.com/etcd-io/etcd/blob/3ead91ca3edf66112d56c453169343515bba71c3/etcdctl/ctlv3/command/ep_command.go#L89
func (c *Client) Health(ctx context.Context) error {
	_, err := c.client.Get(ctx, "health")

	// permission denied is OK since proposal goes through consensus to get it
	if err == nil || errors.Is(err, rpctypes.ErrPermissionDenied) {
		return nil
	}

	return err

}

// Write tries to write a new value with a given key and returns indicator if write operation succeed.
func (c *Client) Write(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {

	leaseResp, err := c.client.Lease.Grant(ctx, int64(ttl.Seconds()))

	if err != nil {
		return false, fmt.Errorf("can't get TTL lease: %w", err)
	}

	// always use notFound guard because otherwise
	// library builds PUT request which is not implemented
	// in the kine
	txnResp, err := c.client.KV.Txn(ctx).If(
		notFound(key),
	).Then(
		clientv3.OpPut(key, value,
			clientv3.WithLease(leaseResp.ID),
		),
	).Commit()
	if err != nil {
		return false, fmt.Errorf("can't write to etcd: %w", err)
	}
	return txnResp.Succeeded, nil
}

func (c *Client) Read(ctx context.Context, key string) (*clientv3.GetResponse, error) {
	resp, err := c.client.KV.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("can't read from etcd: %w", err)
	}
	return resp, nil
}

func notFound(key string) clientv3.Cmp {
	return clientv3.Compare(clientv3.ModRevision(key), "=", 0)
}
