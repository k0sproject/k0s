// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEtcdLeaveCmd(t *testing.T) {
	t.Run("rejects_IP_address_as_arg", func(t *testing.T) {
		leaveCmd := etcdLeaveCmd()
		leaveCmd.SetArgs([]string{"255.255.255.255"})
		err := leaveCmd.Execute()
		assert.ErrorContains(t, err, `unknown command "255.255.255.255" for "leave"`)
	})

	t.Run("rejects_invalid_peer_addresses", func(t *testing.T) {
		leaveCmd := etcdLeaveCmd()
		leaveCmd.SetArgs([]string{"--peer-address=neither/ip/nor/name"})
		err := leaveCmd.Execute()
		assert.ErrorContains(t, err, `invalid argument "neither/ip/nor/name" for "--peer-address" flag: neither an IP address nor a DNS name`)
	})

	t.Run("peer_address_usage_string", func(t *testing.T) {
		leaveCmd := etcdLeaveCmd()
		usageLines := strings.Split(leaveCmd.UsageString(), "\n")
		assert.Contains(t, usageLines, "      --peer-address ip-or-dns-name   etcd peer address to remove (default <this node's peer address>)")
	})
}
