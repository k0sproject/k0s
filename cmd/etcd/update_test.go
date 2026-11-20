// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/k0scontext"
	"github.com/k0sproject/k0s/pkg/etcd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEtcdUpdateCmd(t *testing.T) {
	failure := errors.New("etcd unavailable")
	for _, tt := range []struct {
		name               string
		args               []string
		members            []etcd.Member
		listErr, updateErr error
		wantURL, wantErr   string
		wantID             uint64
	}{
		{name: "by_name", args: []string{"--member-name=member/one", "10.0.0.2"}, members: []etcd.Member{{ID: 42, Name: "member/one"}}, wantID: 42, wantURL: "https://10.0.0.2:2380"},
		{name: "by_old_address", args: []string{"--peer-address=10.0.0.1", "new.example.org"}, members: []etcd.Member{{ID: 7, PeerURL: "https://10.0.0.9:2380"}, {ID: 42, PeerURL: "https://10.0.0.1:2380"}}, wantID: 42, wantURL: "https://new.example.org:2380"},
		{name: "ipv6", args: []string{"--peer-address=2001:db8::1", "2001:db8::2"}, members: []etcd.Member{{ID: 42, PeerURL: "https://[2001:db8::1]:2380"}}, wantID: 42, wantURL: "https://[2001:db8::2]:2380"},
		{name: "configured_address", args: []string{"new.example.org"}, members: []etcd.Member{{ID: 42, PeerURL: "https://10.0.0.1:2380"}}, wantID: 42, wantURL: "https://new.example.org:2380"},
		{name: "not_found", args: []string{"--member-name=missing", "10.0.0.2"}, wantErr: "found 0"},
		{name: "ambiguous_name", args: []string{"--member-name=duplicate", "10.0.0.2"}, members: []etcd.Member{{ID: 1, Name: "duplicate"}, {ID: 2, Name: "duplicate"}}, wantErr: "found 2"},
		{name: "list_error", args: []string{"--member-name=one", "10.0.0.2"}, listErr: failure, wantErr: "can't find etcd member"},
		{name: "update_error", args: []string{"--member-name=one", "10.0.0.2"}, members: []etcd.Member{{ID: 42, Name: "one"}}, updateErr: failure, wantID: 42, wantURL: "https://10.0.0.2:2380", wantErr: "can't update etcd member 2a"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeEtcdMemberUpdateClient{fakeEtcdMemberListClient: fakeEtcdMemberListClient{members: tt.members, listErr: tt.listErr}, updateErr: tt.updateErr}
			cmd := etcdUpdateCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.Flags().String("config", "", "test configuration")
			cmd.SetIn(strings.NewReader("spec: {storage: {type: etcd, etcd: {peerAddress: 10.0.0.1}}}"))
			cmd.SetArgs(append([]string{"--config=-"}, tt.args...))
			err := cmd.ExecuteContext(k0scontext.WithValue[etcdMemberUpdateClient](t.Context(), client))
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			if tt.listErr != nil || tt.updateErr != nil {
				assert.ErrorIs(t, err, failure)
			}
			assert.True(t, client.closed)
			assert.True(t, cmd.SilenceUsage)
			assert.Equal(t, tt.wantID, client.updatedID)
			assert.Equal(t, tt.wantURL, client.updatedURL)
			assert.Equal(t, tt.wantURL != "", client.updated)
		})
	}
}

func TestEtcdUpdateInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		args    []string
		wantErr string
	}{
		{[]string{}, "accepts 1 arg(s), received 0"},
		{[]string{"one", "two"}, "accepts 1 arg(s), received 2"},
		{[]string{"some/path"}, "invalid new peer address"},
		{[]string{"https://example.org:2380"}, "invalid new peer address"},
		{[]string{"host:2380"}, "invalid new peer address"},
		{[]string{"--peer-address=some/path", "new.example.org"}, "invalid argument"},
		{[]string{"--member-name=", "new.example.org"}, "member name must not be empty"},
		{[]string{"--member-name=one", "--peer-address=old.example.org", "new.example.org"}, "none of the others can be"},
	} {
		t.Run("invalid_"+strings.Join(tt.args, " "), func(t *testing.T) {
			client := &fakeEtcdMemberUpdateClient{}
			cmd := etcdUpdateCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)
			require.ErrorContains(t, cmd.ExecuteContext(k0scontext.WithValue[etcdMemberUpdateClient](t.Context(), client)), tt.wantErr)
			assert.False(t, cmd.SilenceUsage, "RunE must not be entered")
			assert.False(t, client.closed, "validation must precede client acquisition")
			assert.False(t, client.updated)
		})
	}
}

type fakeEtcdMemberUpdateClient struct {
	fakeEtcdMemberListClient
	updated    bool
	updatedID  uint64
	updatedURL string
	updateErr  error
}

func (c *fakeEtcdMemberUpdateClient) SetPeerURL(_ context.Context, id uint64, peerURL string) error {
	if c.closed {
		return errors.New("already closed")
	}
	c.updated = true
	c.updatedID, c.updatedURL = id, peerURL
	return c.updateErr
}
