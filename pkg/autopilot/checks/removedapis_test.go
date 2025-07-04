// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package checks

import (
	"cmp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRemovedGVKs(t *testing.T) {
	assert.True(t, slices.IsSortedFunc(removedGVKs[:], func(l, r removedAPI) int {
		if cmp := cmp.Compare(l.group, r.group); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(l.version, r.version); cmp != 0 {
			return cmp
		}
		return cmp.Compare(l.kind, r.kind)
	}), "removedGVKs needs to be sorted, so that it can be used for binary searches")

	// Test two random entries at the top and the bottom of the list
	version, currentVersion := removedInVersion(schema.GroupVersionKind{
		Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta2", Kind: "FlowSchema",
	})
	assert.Equal(t, "v1.29.0", version)
	assert.Equal(t, "v1beta3", currentVersion)

	version, currentVersion = removedInVersion(schema.GroupVersionKind{
		Group: "k0s.k0sproject.example.com", Version: "v1beta1", Kind: "RemovedCRD",
	})
	assert.Equal(t, "v99.99.99", version)
	assert.Empty(t, currentVersion)

	version, currentVersion = removedInVersion(schema.GroupVersionKind{
		Group: "k0s.k0sproject.example.com", Version: "v1beta1", Kind: "MustFail",
	})
	assert.Empty(t, version)
	assert.Empty(t, currentVersion)
}
