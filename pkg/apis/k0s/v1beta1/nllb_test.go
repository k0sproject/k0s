// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeLocalLoadBalancing_IsEnabled(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		nllb    *NodeLocalLoadBalancing
	}{
		{"nil", false, nil},
		{"default", false, &NodeLocalLoadBalancing{}},
		{"true", true, &NodeLocalLoadBalancing{Enabled: true}},
		{"false", false, &NodeLocalLoadBalancing{Enabled: false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.enabled, test.nllb.IsEnabled())
		})
	}
}

func TestNodeLocalLoadBalancing_Unmarshal_ImageOverride(t *testing.T) {
	yamlData := []byte(`
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  images:
    repository: example.com
`)

	c, err := ConfigFromBytes(yamlData)
	require.NoError(t, err)
	errors := c.Validate()
	require.Nil(t, errors)

	nllb := c.Spec.Network.NodeLocalLoadBalancing
	require.NotNil(t, nllb.EnvoyProxy)
	require.NotNil(t, nllb.EnvoyProxy.Image)
	require.Contains(t, nllb.EnvoyProxy.Image.Image, "example.com/")
}

func TestEnvoyProxyImage_Unmarshal(t *testing.T) {
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: TestEnvoyProxyImage_Unmarshal
spec:
  network:
    nodeLocalLoadBalancing:
      envoyProxy:
        image:
          %s: %s
`

	for _, field := range []string{"image", "version"} {
		t.Run(field+"_empty", func(t *testing.T) {
			c, err := ConfigFromBytes(fmt.Appendf(nil, yamlData, field, `""`))
			require.NoError(t, err)
			require.NotNil(t, c)
			require.Empty(t, c.Validate())
			require.NotNil(t, c.Spec)
			require.NotNil(t, c.Spec.Network)
			require.NotNil(t, c.Spec.Network.NodeLocalLoadBalancing)
			require.NotNil(t, c.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy)
			assert.Equal(t, DefaultEnvoyProxyImage(), c.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image)
		})
	}

	for _, test := range []struct {
		field, value, err string
	}{
		{
			"image", `" "`,
			`network: nodeLocalLoadBalancing.envoyProxy.image.image: Invalid value: " ": must not have leading or trailing whitespace`,
		},
		{
			"version", `"*"`,
			`network: nodeLocalLoadBalancing.envoyProxy.image.version: Invalid value: "*": must match regular expression: ^[\w][\w.-]{0,127}(?:@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,})?$`,
		},
	} {
		t.Run(test.field+"_invalid", func(t *testing.T) {
			c, err := ConfigFromBytes(fmt.Appendf(nil, yamlData, test.field, test.value))
			require.NoError(t, err)
			require.NotNil(t, c)
			errs := c.Validate()
			require.Len(t, errs, 1)
			assert.ErrorContains(t, errs[0], test.err)
		})
	}
}
