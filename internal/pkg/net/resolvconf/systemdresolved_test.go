// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package resolvconf_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSystemdResolvedStub(t *testing.T) {
	t.Run("nonexistent_file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "resolv.conf")
		detected, err := resolvconf.IsSystemdResolvedStub(path)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.False(t, detected)
	})

	for _, test := range []struct {
		name     string
		content  string
		expected bool
	}{
		{"empty_file", "", false},
		{"no_nameservers", "search example.com\n", false},
		{"whitespace", "  nameserver\t127.0.0.53   ", false}, // no whitespace allowed in front of keywords
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
			path := filepath.Join(t.TempDir(), "resolv.conf")
			require.NoError(t, os.WriteFile(path, []byte(test.content), 0644))
			detected, err := resolvconf.IsSystemdResolvedStub(path)
			if assert.NoError(t, err) {
				assert.Equal(t, test.expected, detected)
			}
		})
	}
}
