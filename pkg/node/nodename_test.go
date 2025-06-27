// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"runtime"
	"testing"

	apitypes "k8s.io/apimachinery/pkg/types"
	nodeutil "k8s.io/component-helpers/node/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNodeName(t *testing.T) {
	t.Run("should_always_return_override_if_given", func(t *testing.T) {
		name, err := GetNodeName("override")
		if assert.NoError(t, err) {
			assert.Equal(t, apitypes.NodeName("override"), name)
		}
	})

	if runtime.GOOS != "windows" {
		kubeHostname, err := nodeutil.GetHostname("")
		require.NoError(t, err)

		t.Run("should_call_kubernetes_hostname_helper_on_linux", func(t *testing.T) {
			name, err := GetNodeName("")
			if assert.NoError(t, err) {
				assert.Equal(t, apitypes.NodeName(kubeHostname), name)
			}
		})
	}
}
