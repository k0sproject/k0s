// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"net"
	"strings"
	"testing"
	"text/template"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeepalivedVRRPConfigTemplate(t *testing.T) {
	templ, err := template.New("vrrp").Parse(KeepalivedVRRPConfigTemplate)
	require.NoError(t, err)

	t.Run("quotes the password", func(t *testing.T) {
		var rendered strings.Builder
		err := templ.Execute(&rendered, &keepalivedConfig{
			VRRPInstances: []k0sAPI.VRRPInstance{{AuthPass: "fake #!"}},
		})

		require.NoError(t, err)
		assert.Contains(t, rendered.String(), "\n        auth_pass \"fake #!\"\n")
	})

	t.Run("quotes the interface", func(t *testing.T) {
		var rendered strings.Builder
		err := templ.Execute(&rendered, &keepalivedConfig{
			VRRPInstances: []k0sAPI.VRRPInstance{{Interface: "fake#nic!0"}},
		})

		require.NoError(t, err)
		assert.Contains(t, rendered.String(), "\n    interface \"fake#nic!0\"\n")
	})
}

func TestEscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It\'s a "nice" \'test\'`, escapeSingleQuotes(`It's a "nice" \'test'`))
}

func TestProxyTarget(t *testing.T) {
	for _, test := range []struct {
		name, address, vip string
		want               []string
	}{
		{"IPv4", "10.0.0.1", "10.0.0.100", []string{"-j", "DNAT", "--to-destination", "10.0.0.1:6444"}},
		{"IPv6", "fd00::1", "fd00::100", []string{"-j", "DNAT", "--to-destination", "[fd00::1]:6444"}},
		{"other family", "10.0.0.1", "fd00::100", []string{"-j", "REDIRECT", "--to-port", "6444"}},
		{"no address", "", "10.0.0.100", []string{"-j", "REDIRECT", "--to-port", "6444"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			k := &Keepalived{Config: &k0sAPI.KeepalivedSpec{UserSpaceProxyPort: 6444}, Address: test.address}
			assert.Equal(t, test.want, k.proxyTarget(net.ParseIP(test.vip)))
		})
	}
}
