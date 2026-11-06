// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cplb

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	k0sAPI "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeSingleQuotes(t *testing.T) {
	assert.Equal(t, `It\'s a "nice" \'test\'`, escapeSingleQuotes(`It's a "nice" \'test'`))
}

func TestVirtualServersTemplate(t *testing.T) {
	templ, err := template.New("keepalived-virtualservers").Parse(KeepalivedVirtualServersConfigTemplate)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, templ.Execute(&buf, &keepalivedConfig{
		VirtualServers:        []k0sAPI.VirtualServer{{IPAddress: "10.0.0.100", PersistenceTimeoutSeconds: 360, Konnectivity: true}},
		RealServers:           []string{"10.0.0.1"},
		APIServerPort:         6443,
		KonnectivityAgentPort: 8132,
		K0sAPIPort:            9443,
	}))
	config := strings.Join(strings.Fields(buf.String()), " ")

	assert.Contains(t, config, "virtual_server 10.0.0.100 6443 { delay_loop 0 lb_algo lb_kind persistence_timeout 360 protocol TCP real_server 10.0.0.1 6443 {")
	assert.Contains(t, config, "virtual_server 10.0.0.100 8132 { delay_loop 0 lb_algo lb_kind protocol TCP real_server 10.0.0.1 8132 {")
	assert.NotContains(t, config, "9443")
}
