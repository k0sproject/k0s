//go:build unix

// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCacheOptions(t *testing.T) {
	opts := workerCacheOptions("some-node")

	require.Len(t, opts.ByObject, 1)
	for obj, byObject := range opts.ByObject {
		assert.IsType(t, &corev1.Node{}, obj)
		require.NotNil(t, byObject.Field)
		assert.Equal(t, "metadata.name=some-node", byObject.Field.String())
		assert.Nil(t, byObject.Label)
	}
}
