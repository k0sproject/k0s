// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/k0sproject/k0s/internal/pkg/net/resolvconf"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTaints(t *testing.T) {
	cases := []struct {
		name          string
		spec          string
		expectedTaint corev1.Taint
		expectedErr   bool
	}{
		{
			name:        "invalid spec format",
			spec:        "",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc=xyz:NoSchedule",
			expectedErr: true,
		},
		{
			name:        "invalid spec format",
			spec:        "foo=abc:xyz:NoSchedule",
			expectedErr: true,
		},
		{
			name:        "invalid spec effect",
			spec:        "foo=abc:invalid_effect",
			expectedErr: true,
		},
		{
			name: "full taint",
			spec: "foo=abc:NoSchedule",
			expectedTaint: corev1.Taint{
				Key:    "foo",
				Value:  "abc",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectedErr: false,
		},
	}

	for _, c := range cases {
		taint, err := parseTaint(c.spec)
		if c.expectedErr && err == nil {
			t.Errorf("[%s] expected error for spec %s, but got nothing", c.name, c.spec)
		}
		if !c.expectedErr && err != nil {
			t.Errorf("[%s] expected no error for spec %s, but got: %v", c.name, c.spec, err)
		}
		require.Equal(t, c.expectedTaint, taint)
	}
}

func TestUseSystemdResolvedUplink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Not used on Windows")
	}

	writeFile := func(t *testing.T, root, name, content string) {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	t.Run("without systemd-resolved", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, resolvconf.Path, "nameserver 127.0.0.53\n")

		useUplink, err := useSystemdResolvedUplink(root)
		require.NoError(t, err)
		assert.False(t, useUplink)
	})

	for _, test := range []struct {
		name, content string
		expected      bool
	}{
		{"stub", "nameserver 127.0.0.53\n", true},
		{"no nameservers", "search example.com\n", true},
		{"other nameservers", "nameserver 1.2.3.4\n", false},
	} {
		t.Run("with systemd-resolved and "+test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
			writeFile(t, root, resolvconf.Path, test.content)

			useUplink, err := useSystemdResolvedUplink(root)
			require.NoError(t, err)
			assert.Equal(t, test.expected, useUplink)
		})
	}

	t.Run("with systemd-resolved and nonexistent resolv.conf", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")

		useUplink, err := useSystemdResolvedUplink(root)
		require.NoError(t, err)
		assert.True(t, useUplink)
	})

	t.Run("with systemd-resolved and resolv.conf being the uplink file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
		resolvConfPath := filepath.Join(root, resolvconf.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(resolvConfPath), 0700))
		require.NoError(t, os.Symlink(filepath.Join(root, resolvconf.SystemdResolvedUplinkPath), resolvConfPath))

		useUplink, err := useSystemdResolvedUplink(root)
		require.NoError(t, err)
		assert.False(t, useUplink)
	})

	t.Run("with systemd-resolved and unreadable resolv.conf", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, resolvconf.SystemdResolvedUplinkPath, "nameserver 1.2.3.4\n")
		require.NoError(t, os.MkdirAll(filepath.Join(root, resolvconf.Path), 0700)) // reading a directory fails

		useUplink, err := useSystemdResolvedUplink(root)
		assert.ErrorContains(t, err, "is a directory")
		assert.False(t, useUplink)
	})
}
