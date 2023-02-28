/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
