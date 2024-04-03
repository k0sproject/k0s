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

package updater

import (
	"testing"
)

func TestVersion_Compare(t *testing.T) {

	tests := []struct {
		name         string
		version      Version
		otherVersion string
		want         int
	}{
		{
			name:         "different major",
			version:      "v1.23.3+k0s.0",
			otherVersion: "v2.23.3+k0s.0",
			want:         -1,
		},
		{
			name:         "different minor",
			version:      "v1.23.3+k0s.0",
			otherVersion: "v1.22.3+k0s.0",
			want:         1,
		},
		{
			name:         "different patch",
			version:      "v1.23.3+k0s.0",
			otherVersion: "v1.23.4+k0s.0",
			want:         -1,
		},
		{
			name:         "different k0s patch",
			version:      "v1.23.3+k0s.0",
			otherVersion: "v1.23.3+k0s.1",
			want:         -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.Compare(tt.otherVersion); got != tt.want {
				t.Errorf("Version.Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
