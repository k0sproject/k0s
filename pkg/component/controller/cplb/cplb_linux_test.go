// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"text/template"
	"time"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	t.Run("escapes single quotes in notify script paths", func(t *testing.T) {
		var rendered strings.Builder
		err := templ.Execute(&rendered, &keepalivedConfig{
			VRRPInstances:    []k0sAPI.VRRPInstance{{}},
			IPVSLoadBalancer: true,
			k0sBin:           "/fake/it's/k0s",
			runDir:           "/fake/run dir",
		})

		require.NoError(t, err)
		assert.Contains(t, rendered.String(), `
    notify_master "'/fake/it\'s/k0s' keepalived-setstate -r '/fake/run dir' -s MASTER"
    notify_backup "'/fake/it\'s/k0s' keepalived-setstate -r '/fake/run dir' -s BACKUP"
`)
	})

	for _, test := range []struct{ name, path string }{
		{"a double quote", `/fake/"k0s"`},
		{"a backslash", `/fake\k0s`},
		{"a newline", "/fake\nk0s"},
		{"a NUL character", "/fake\x00k0s"},
		{"an unbraced keepalived definition", "/fake/$_ENV /k0s"},
		{"a braced keepalived definition", "/fake/${_ENV}/k0s"},
	} {
		t.Run("rejects notify script paths with "+test.name, func(t *testing.T) {
			for _, config := range []keepalivedConfig{
				{k0sBin: test.path, runDir: "/fake/run"},
				{k0sBin: "/fake/k0s", runDir: test.path},
			} {
				config.VRRPInstances = []k0sAPI.VRRPInstance{{}}
				config.IPVSLoadBalancer = true

				err := templ.Execute(io.Discard, &config)

				assert.ErrorContains(t, err, fmt.Sprintf("can't use %q in a keepalived notify script", test.path))
			}
		})
	}

	t.Run("accepts any paths without notify scripts", func(t *testing.T) {
		err := templ.Execute(io.Discard, &keepalivedConfig{
			VRRPInstances: []k0sAPI.VRRPInstance{{}},
			k0sBin:        `/fake/"k0s"`,
			runDir:        `/fake/"run"`,
		})

		assert.NoError(t, err, "Paths that aren't rendered shouldn't be checked")
	})
}

func TestKeepalivedVirtualServersConfigTemplate(t *testing.T) {
	templ, err := template.New("virtualservers").Parse(KeepalivedVirtualServersConfigTemplate)
	require.NoError(t, err)

	t.Run("delayLoop", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			delayLoop time.Duration
			rendered  string
		}{
			{"whole seconds", time.Minute, "60.000000"},
			{"fractions of a second", 1500 * time.Millisecond, "1.500000"},
			{"a few microseconds", 50 * time.Microsecond, "0.000050"},
		} {
			t.Run(test.name, func(t *testing.T) {
				var rendered strings.Builder
				err := templ.Execute(&rendered, &keepalivedConfig{
					VirtualServers: []k0sAPI.VirtualServer{{DelayLoop: metav1.Duration{Duration: test.delayLoop}}},
					RealServers:    []string{"192.0.2.1"},
				})

				require.NoError(t, err)
				assert.Contains(t, rendered.String(), "\n    delay_loop "+test.rendered+"\n")
			})
		}
	})
}

func TestCustomKeepalivedTemplatePaths(t *testing.T) {
	templ, err := template.New("custom").Parse(`{{ .K0sBin }} {{ .RunDir }}`)
	require.NoError(t, err)

	var rendered strings.Builder
	err = templ.Execute(&rendered, &keepalivedConfig{
		k0sBin: `/fake/it's/"k0s"`,
		runDir: `/fake\run/${_ENV}`,
	})

	require.NoError(t, err, "K0sBin and RunDir shouldn't be checked")
	assert.Equal(t, `/fake/it\'s/"k0s" /fake\run/${_ENV}`, rendered.String(), "Only single quotes should be escaped")
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
