// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k0sproject/k0s/pkg/autopilot/common"
	"github.com/k0sproject/k0s/pkg/node"
)

func TestFindEffectiveHostname(t *testing.T) {
	t.Run("falls back to the OS hostname when unset", func(t *testing.T) {
		t.Setenv("AUTOPILOT_HOSTNAME", "")
		got, err := common.FindEffectiveHostname()
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	})

	t.Run("honors the AUTOPILOT_HOSTNAME env var", func(t *testing.T) {
		t.Setenv("AUTOPILOT_HOSTNAME", "env-hostname")
		got, err := common.FindEffectiveHostname()
		require.NoError(t, err)
		assert.Equal(t, "env-hostname", got)
	})
}

func TestFindKubeletHostname(t *testing.T) {
	t.Run("uses the hostname-override extra arg", func(t *testing.T) {
		got := common.FindKubeletHostname("--hostname-override=custom-host --other-flag=1")
		assert.Equal(t, "custom-host", got)
	})

	t.Run("falls back to the default node name when no override arg is set", func(t *testing.T) {
		defaultNodeName, err := node.GetNodeName("")
		require.NoError(t, err)

		got := common.FindKubeletHostname("--other-flag=1")
		assert.Equal(t, string(defaultNodeName), got)
	})

	t.Run("falls back to the default node name when kubeletExtraArgs is empty", func(t *testing.T) {
		defaultNodeName, err := node.GetNodeName("")
		require.NoError(t, err)

		got := common.FindKubeletHostname("")
		assert.Equal(t, string(defaultNodeName), got)
	})
}
