/*
Copyright 2021 k0s authors

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

package stringmap

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToArgs(t *testing.T) {
	tests := []struct {
		name string
		args StringMap
		want []string
	}{
		{
			"basic",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			[]string{"foo=bar", "bar=baf", "empty="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.ToArgs()
			sort.Strings(got)
			sort.Strings(tt.want)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMerge(t *testing.T) {
	original := StringMap{
		"foo": "bar",
	}

	original.Merge(StringMap{
		"another": "val",
		"foo":     "overridden",
	})

	assert.Equal(t, "overridden", original["foo"])
	assert.Equal(t, "val", original["another"])
}

func TestEquals(t *testing.T) {
	tests := []struct {
		name  string
		this  StringMap
		other StringMap
		want  bool
	}{
		{
			"basic",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			StringMap{
				"bar":   "baf",
				"foo":   "bar",
				"empty": "",
			},
			true,
		},
		{
			"nils",
			nil,
			nil,
			true,
		},
		{
			"different len",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			StringMap{
				"bar": "baf",
				"foo": "bar",
			},
			false,
		},
		{
			"different vals",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			StringMap{
				"bar":   "baf",
				"foo":   "bar",
				"empty": "was empty - not anymore :)",
			},
			false,
		},
		{
			"non-nil vs. nil",
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			nil,
			false,
		},
		{
			"nil vs. non-nil",
			nil,
			StringMap{
				"foo":   "bar",
				"bar":   "baf",
				"empty": "",
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.this.Equals(tt.other)
			assert.Equal(t, tt.want, got)
		})
	}
}
