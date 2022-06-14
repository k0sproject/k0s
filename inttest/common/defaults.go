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
	TargetK0sVersion = "v1.23.3+k0s.1"
)

type K0sVersion string
type K0sVersionedPlatformResourceMap map[K0sVersion]PlatformedResourceMap
type PlatformedResourceMap map[string]ResourceMap
type ResourceMap map[string]AttributeMap
type AttributeMap map[string]string

var Versions = K0sVersionedPlatformResourceMap{
	"v1.23.3+k0s.1": {
		"linux-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.1/k0s-v1.23.3+k0s.1-amd64",
				"sha256": "0cd1f7c49ef81e18d3873a77ccabb5e4095db1c3647ca3fa8fc3eb16566e204e",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.1/k0s-airgap-bundle-v1.23.3+k0s.1-amd64",
				"sha256": "258f3edd0c260a23c579406f5cc04a599a6f59cc1707f9bd523d7a9abc07f0e2",
			},
		},
		"linux-arm64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.1/k0s-v1.23.3+k0s.1-arm64",
				"sha256": "350adde6c452abd56a3c8113bf5af254fc17bcc41946e32ae47b580626a9293c",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.1/k0s-airgap-bundle-v1.23.3+k0s.1-arm64",
				"sha256": "2fe4976a90193e2ec89d3cff4ebf6bd063b38cf23116071689415fd19f251f29",
			},
		},
		"windows-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.23.3%2Bk0s.1/k0s-v1.23.3+k0s.1-amd64.exe",
				"sha256": "f9e064f70c997e55dacbd3b36ca04029bb7995e84be8084d8bbd2cd75601fe30",
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
