// Copyright 2022 k0s authors
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

package common

import (
	"runtime"

	v1 "k8s.io/api/core/v1"
)

const (
	TargetK0sVersion = "v1.23.8+k0s.0"
)

type K0sVersion string
type K0sVersionedPlatformResourceMap map[K0sVersion]PlatformedResourceMap
type PlatformedResourceMap map[string]ResourceMap
type ResourceMap map[string]AttributeMap
type AttributeMap map[string]string

var Versions = K0sVersionedPlatformResourceMap{
	"v1.23.8+k0s.0": {
		"linux-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.8%2Bk0s.0/k0s-v1.23.8+k0s.0-amd64",
				"sha256": "8b955202e923612f6196bf3eaea7744f56347a5494b4ffe8c2d4618212193383",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.8%2Bk0s.0/k0s-airgap-bundle-v1.23.8+k0s.0-amd64",
				"sha256": "5db2c1d3c7ff3e308eae1073a33f18a415e3096b4f901a3b4fe01ea568f18259",
			},
		},
		"linux-arm64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.8%2Bk0s.0/k0s-v1.23.8+k0s.0-arm64",
				"sha256": "9ec9dc3aa4e322335e304f375a1820b86e0a2199245a7a811cddb594f83c7786",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.8%2Bk0s.0/k0s-airgap-bundle-v1.23.8+k0s.0-arm64",
				"sha256": "193d9ef219db80f9e3b5b65afc92d51509ec1f95a3300cd48f646c7cbe7288e9",
			},
		},
		"windows-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.8%2Bk0s.0/k0s-v1.23.8+k0s.0-amd64.exe",
				"sha256": "5a9dab6c2e34f291c3aee4b9be664e9bbdcefcebd8422ca869cd80071b92186f",
			},
			// no airgap bundles published for windows
		},
	},
}

// DefaultNodeLabels creates a default map of labels expected to be seen
// on every signal node.
func DefaultNodeLabels() map[string]string {
	return map[string]string{
		v1.LabelOSStable:   runtime.GOOS,
		v1.LabelArchStable: runtime.GOARCH,
	}
}
