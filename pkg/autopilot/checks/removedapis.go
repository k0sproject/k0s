// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package checks

import (
	"cmp"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type removedAPI struct {
	group, version, kind, removedInK8sVersion string
	// currentAPIVersion declares a version that is still supported for the Group Kind.
	// If it's empty, it means that the Group Kind is removed in the removedInVersion.
	currentAPIVersion string
}

// If candidate has been removed, returns the kubernetes version in which it was removed
// and the current version for Group Kind.
func removedInVersion(candidate schema.GroupVersionKind) (string, string) {
	if idx, found := sort.Find(len(removedGVKs), func(i int) int {
		if cmp := cmp.Compare(candidate.Group, removedGVKs[i].group); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(candidate.Version, removedGVKs[i].version); cmp != 0 {
			return cmp
		}
		return cmp.Compare(candidate.Kind, removedGVKs[i].kind)
	}); found {
		return removedGVKs[idx].removedInK8sVersion, removedGVKs[idx].currentAPIVersion
	}

	return "", ""
}

// Sorted array of removed APIs.
var removedGVKs = [...]removedAPI{
	{"flowcontrol.apiserver.k8s.io", "v1beta2", "FlowSchema", "v1.29.0", "v1beta3"},
	{"flowcontrol.apiserver.k8s.io", "v1beta2", "PriorityLevelConfiguration", "v1.29.0", "v1"},
	{"flowcontrol.apiserver.k8s.io", "v1beta3", "FlowSchema", "v1.32.0", "v1"},
	{"flowcontrol.apiserver.k8s.io", "v1beta3", "PriorityLevelConfiguration", "v1.32.0", "v1"},
	{"k0s.k0sproject.example.com", "v1beta1", "RemovedCRD", "v99.99.99", ""}, // This is a test entry
}
