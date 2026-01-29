// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEtcdUpdateCmd(t *testing.T) {
	t.Run("rejects_invalid_peer_argument", func(t *testing.T) {
		updateCmd := etcdUpdateCmd()
		updateCmd.SetArgs([]string{"some/path"})
		err := updateCmd.Execute()
		assert.ErrorContains(t, err, `"some/path" neither an IP address nor a DNS name`)
	})

	t.Run("requires_minimum_arguments", func(t *testing.T) {
		updateCmd := etcdUpdateCmd()
		updateCmd.SetArgs([]string{})
		err := updateCmd.Execute()
		assert.ErrorContains(t, err, "requires at least 1 arg(s)")
	})

	t.Run("rejects_invalid_peer_address_flag", func(t *testing.T) {
		updateCmd := etcdUpdateCmd()
		updateCmd.SetArgs([]string{"--peer-address=neither/ip/nor/name", "peer1"})
		err := updateCmd.Execute()
		assert.ErrorContains(t, err, `invalid argument "neither/ip/nor/name" for "--peer-address" flag: neither an IP address nor a DNS name`)
	})

	t.Run("rejects_invalid_member_name_flag", func(t *testing.T) {
		updateCmd := etcdUpdateCmd()
		updateCmd.SetArgs([]string{"--member-name=neither/ip/nor/name", "peer1"})
		err := updateCmd.Execute()
		assert.ErrorContains(t, err, `invalid argument "neither/ip/nor/name" for "--member-name" flag: neither an IP address nor a DNS name`)
	})

	t.Run("usage_string_contains_flag_help", func(t *testing.T) {
		updateCmd := etcdUpdateCmd()
		usageLines := strings.Split(updateCmd.UsageString(), "\n")
		assert.Contains(t, usageLines, "      --peer-address ip-or-dns-name   etcd peer address to update (default <this node's peer address>)")
		assert.Contains(t, usageLines, "      --member-name ip-or-dns-name    etcd member name to update")
	})
}
