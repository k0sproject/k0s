// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/k0sproject/k0s/cmd"
	"github.com/k0sproject/k0s/pkg/featuregate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ValidateDefaultConfig(t *testing.T) {
	t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

	var defaultConfigBytes bytes.Buffer
	configCreateCmd := cmd.NewRootCmd()
	configCreateCmd.SetArgs([]string{"config", "create"})
	configCreateCmd.SetOut(&defaultConfigBytes)
	require.NoError(t, configCreateCmd.Execute())

	configValidateCmd := cmd.NewRootCmd()
	configValidateCmd.SetArgs([]string{"config", "validate", "-c", "-"})
	configValidateCmd.SetIn(bytes.NewReader(defaultConfigBytes.Bytes()))
	require.NoError(t, configValidateCmd.Execute())
}

func TestConfig_ValidateIPV6SingleStack(t *testing.T) {
	config := []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  network:
    podCIDR: fd00::/108
    serviceCIDR: fd01::/108
`)

	t.Run("WithoutFeatureGates", func(t *testing.T) {
		t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

		expected := `spec: network: podCIDR: Invalid value: "fd00::/108": feature gate IPv6SingleStack must be explicitly enabled to use IPv6 single stack`

		var stderr strings.Builder
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"config", "validate", "-c", "-"})
		underTest.SetIn(bytes.NewReader(config))
		underTest.SetErr(&stderr)
		assert.ErrorContains(t, underTest.Execute(), expected)
		assert.Equal(t, "Error: "+expected+"\n", stderr.String())
	})

	t.Run("WithFeatureGate", func(t *testing.T) {
		t.Cleanup(func() { featuregate.FlushDefaultFeatureGates(t) })

		var stderr strings.Builder
		underTest := cmd.NewRootCmd()
		underTest.SetArgs([]string{"config", "validate", "-c", "-", "--feature-gates", "IPv6SingleStack=true"})
		underTest.SetIn(bytes.NewReader(config))
		underTest.SetErr(&stderr)
		assert.NoError(t, underTest.Execute())
		assert.Empty(t, stderr.String())
	})
}
