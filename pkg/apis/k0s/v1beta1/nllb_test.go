/*
Copyright 2022 k0s authors

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
	yamlData := `
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: foobar
spec:
  images:
    repository: example.com
`

	c, err := ConfigFromString(yamlData)
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
			c, err := ConfigFromString(fmt.Sprintf(yamlData, field, `""`))
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
			c, err := ConfigFromString(fmt.Sprintf(yamlData, test.field, test.value))
			require.NoError(t, err)
			require.NotNil(t, c)
			errs := c.Validate()
			require.Len(t, errs, 1)
			assert.ErrorContains(t, errs[0], test.err)
		})
	}
}
