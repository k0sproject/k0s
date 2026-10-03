// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package resolvconf_test

import (
	"strings"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"
	"github.com/stretchr/testify/assert"
)

func TestIsSystemdResolvedStub(t *testing.T) {
	for _, test := range []struct {
		name, content string
	}{
		{"empty_file", ""},
		{"no_nameservers", "search example.com\n"},
		{"whitespace", "  nameserver\t127.0.0.53   "}, // no whitespace allowed in front of keywords
	} {
		t.Run(test.name, func(t *testing.T) {
			detected, err := resolvconf.IsSystemdResolvedStub(strings.NewReader(test.content))
			assert.Equal(t, err, resolvconf.ErrNoNameservers)
			assert.False(t, detected)
		})
	}

	for _, test := range []struct {
		name     string
		content  string
		expected bool
	}{
		{"trailing_nonsense", "nameserver\t127.0.0.53  you won't look at me, right?", true},
		{
			"multiple_nameservers_systemd_resolved_first",
			"nameserver 127.0.0.53\nsearch example.com\nnameserver 1.2.3.4",
			false,
		},
		{
			"multiple_nameservers_systemd_resolved_second",
			"nameserver 1.2.3.4\nnameserver 127.0.0.53\nsearch example.com",
			false,
		},
		{
			"zoned_link_local_nameserver_before_systemd_resolved",
			"nameserver fe80::1%eth0\nnameserver 127.0.0.53",
			false,
		},
		{
			"zoned_link_local_nameserver_after_systemd_resolved",
			"nameserver 127.0.0.53\nnameserver fe80::1%eth0",
			false,
		},
		{
			"commented_nameserver",
			"search example.com\nnameserver 127.0.0.53\n#nameserver 1.2.3.4",
			true,
		},
		{
			"comment_after_nameserver",
			"search example.com\nnameserver 127.0.0.53 # not 1.2.3.4",
			true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			detected, err := resolvconf.IsSystemdResolvedStub(strings.NewReader(test.content))
			if assert.NoError(t, err) {
				assert.Equal(t, test.expected, detected)
			}
		})
	}
}
