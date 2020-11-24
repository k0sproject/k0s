/*
Copyright 2020 Mirantis, Inc.

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
package util

import "testing"

func TestStringSliceContains(t *testing.T) {
	type args struct {
		strSlice []string
		str      string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "not contains",
			args: args{
				strSlice: []string{"foo", "bar"},
				str:      "foobar",
			},
			want: false,
		},
		{
			name: "contains",
			args: args{
				strSlice: []string{"foo", "bar"},
				str:      "bar",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StringSliceContains(tt.args.strSlice, tt.args.str); got != tt.want {
				t.Errorf("StringSliceContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArrayIsEqual(t *testing.T) {
	type args struct {
		arrayString     []string
		arrayStringComp []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "unordered equal",
			args: args{
				arrayString:     []string{"a", "b", "c"},
				arrayStringComp: []string{"b", "a", "c"},
			},
			want: true,
		},
		{
			name: "ordered unequal",
			args: args{
				arrayString:     []string{"a", "b", "c"},
				arrayStringComp: []string{"a", "b", "d"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStringArrayEqual(tt.args.arrayString, tt.args.arrayStringComp); got != tt.want {
				t.Errorf("IsStringArrayEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
