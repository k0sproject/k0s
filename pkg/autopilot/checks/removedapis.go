// Copyright 2024 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"cmp"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type removedAPI struct {
	group, version, kind, removedInVersion string
}

// Returns the Kubernetes version in which candidate has been removed, if any.
func removedInVersion(candidate schema.GroupVersionKind) string {
	if idx, found := sort.Find(len(removedGVKs), func(i int) int {
		if cmp := cmp.Compare(candidate.Group, removedGVKs[i].group); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(candidate.Version, removedGVKs[i].version); cmp != 0 {
			return cmp
		}
		return cmp.Compare(candidate.Kind, removedGVKs[i].kind)
	}); found {
		return removedGVKs[idx].removedInVersion
	}

	return ""
}

// Sorted array of removed APIs.
var removedGVKs = [...]removedAPI{
	{"flowcontrol.apiserver.k8s.io", "v1beta2", "FlowSchema", "v1.29.0"},
	{"flowcontrol.apiserver.k8s.io", "v1beta2", "PriorityLevelConfiguration", "v1.29.0"},
	{"flowcontrol.apiserver.k8s.io", "v1beta3", "FlowSchema", "v1.32.0"},
	{"flowcontrol.apiserver.k8s.io", "v1beta3", "PriorityLevelConfiguration", "v1.32.0"},
	{"k0s.k0sproject.example.com", "v1beta1", "RemovedCRD", "v99.99.99"}, // This is a test entry
}
