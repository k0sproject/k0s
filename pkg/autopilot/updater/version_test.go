// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
