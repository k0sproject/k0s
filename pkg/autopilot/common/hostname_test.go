// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package common_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k0sproject/k0s/pkg/autopilot/common"
)

func TestFindEffectiveHostname(t *testing.T) {
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
}
