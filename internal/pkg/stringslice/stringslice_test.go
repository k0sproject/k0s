// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package stringslice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			"all unique",
			[]string{"a", "b", "c"},
			[]string{"a", "b", "c"},
		},
		{
			"some non unique",
			[]string{"c", "a", "b", "b", "c"},
			[]string{"c", "a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.input)
			assert.Equal(t, tt.want, got)

		})
	}
}
