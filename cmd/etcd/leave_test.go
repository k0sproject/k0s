/*
Copyright 2024 k0s authors

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
