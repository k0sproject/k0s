// Copyright 2021 k0s authors
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
	v1 "k8s.io/api/core/v1"
)

const (
	TargetK0sVersion = "v1.24.4+k0s.0"
)

type K0sVersion string
type K0sVersionedPlatformResourceMap map[K0sVersion]PlatformedResourceMap
type PlatformedResourceMap map[string]ResourceMap
type ResourceMap map[string]AttributeMap
type AttributeMap map[string]string

var Versions = K0sVersionedPlatformResourceMap{
	"v1.24.4+k0s.0": {
		"linux-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.24.4+k0s.0/k0s-v1.24.4+k0s.0-amd64",
				"sha256": "c94fb7da760cbdde5ef90e0183cf9c2dd32be139d82e64c3f6ab83d614049383",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.24.4+k0s.0/k0s-airgap-bundle-v1.24.4+k0s.0-amd64",
				"sha256": "7a3e5ccee558f0935ec39b416513a90fa504d1fdf720a17565a2e50d401b9935",
			},
		},
		"linux-arm64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.24.4+k0s.0/k0s-v1.24.4+k0s.0-arm64",
				"sha256": "e0037114f1a36f10c2bf5bba672adb3a29b0aae16f22180317630c03d05ee8d0",
			},
			"airgap": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.24.4+k0s.0/k0s-airgap-bundle-v1.24.4+k0s.0-arm64",
				"sha256": "4980e00a4124ca39842b227c45645ae4c777e62e78be0837b32c798d5192405a",
			},
		},
		"windows-amd64": {
			"k0s": {
				"url":    "https://github.com/k0sproject/k0s/releases/download/v1.24.4+k0s.0/k0s-v1.24.4+k0s.0-amd64.exe",
				"sha256": "c82ec064f1b17465208c6ae235ea5e2c649a8d82a7dec5304c31079ed9c5893b",
			},
			// no airgap bundles published for windows
		},
	},
}

// LinuxAMD64NodeLabels creates a default map of labels expected to be seen on
// a linux-amd64 signal node.
func LinuxAMD64NodeLabels() map[string]string {
	return map[string]string{
		v1.LabelOSStable:   "linux",
		v1.LabelArchStable: "amd64",
	}
}
