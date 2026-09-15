// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/k0sproject/k0s/pkg/applier"
)

func TestToUnstructured(t *testing.T) {
	t.Run("infers GVK from the default scheme when unset", func(t *testing.T) {
		cm := &corev1.ConfigMap{}
		u, err := applier.ToUnstructured(nil, cm)
		require.NoError(t, err)
		assert.Equal(t, "v1", u.GetAPIVersion())
		assert.Equal(t, "ConfigMap", u.GetKind())
	})
}
