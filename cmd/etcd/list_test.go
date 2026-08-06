// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtcdListCmd(t *testing.T) {
	t.Run("lists_members_as_json", func(t *testing.T) {
		client := fakeEtcdMemberListClient{
			members: []etcd.Member{
				{ID: 1, Name: "node-1", PeerURL: "https://10.0.0.1:2380"},
				{ID: 2, Name: "node-2", PeerURL: "https://10.0.0.2:2380"},
			},
		}

		ctx := t.Context()
		ctx = k0scontext.WithValue[etcdMemberListClient](ctx, &client)

		var (
			stdout bytes.Buffer
			stderr strings.Builder
		)
		underTest := etcdListCmd()
		underTest.SetOut(&stdout)
		underTest.SetErr(&stderr)
		err := underTest.ExecuteContext(ctx)
		require.NoError(t, err)
		assert.True(t, client.closed, "expected the etcd client to be closed")
		assert.Empty(t, stderr.String())

		var got struct {
			Members map[string]string `json:"members"`
		}
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
		assert.Equal(t, map[string]string{
			"node-1": "https://10.0.0.1:2380",
			"node-2": "https://10.0.0.2:2380",
		}, got.Members)
	})

	t.Run("wraps_member_list_errors", func(t *testing.T) {
		client := fakeEtcdMemberListClient{
			listErr: errors.New("member list failed"),
		}

		ctx := t.Context()
		ctx = k0scontext.WithValue[etcdMemberListClient](ctx, &client)

		var (
			stdout strings.Builder
			stderr strings.Builder
		)
		underTest := etcdListCmd()
		underTest.SetOut(&stdout)
		underTest.SetErr(&stderr)
		err := underTest.ExecuteContext(ctx)
		assert.ErrorContains(t, err, "can't list etcd cluster members: member list failed")
		assert.True(t, client.closed, "Expected the etcd client to be closed")
		assert.Empty(t, stdout.String())
		assert.Equal(t, "Error: can't list etcd cluster members: member list failed\n", stderr.String())
	})
}

type fakeEtcdMemberListClient struct {
	mu      sync.Mutex
	members []etcd.Member
	listErr error
	closed  bool
}

func (s *fakeEtcdMemberListClient) ListMembers(context.Context) ([]etcd.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New("already closed")
	}

	if s.listErr != nil {
		return nil, s.listErr
	}
	return slices.Clone(s.members), nil
}

func (s *fakeEtcdMemberListClient) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("already closed")
	}

	s.closed = true
	return nil
}
