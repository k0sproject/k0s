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

package controller

import "testing"

func Test_replicaCount(t *testing.T) {
	tests := []struct {
		name  string
		nodes int
		want  int
	}{
		{
			"one replica even for zero nodes",
			0,
			1,
		},
		{
			"one replica for one node",
			1,
			1,
		},
		{
			"2 replicas for two nodes (1 + ceil(2/10)) ",
			2,
			2,
		},
		{
			"2 replicas for 10 nodes (1 + 10/10)",
			10,
			2,
		},
		{
			"3 replicas for 15 nodes (1 + ceil(15/10))",
			15,
			3,
		},
		{
			"3 replicas for 20 nodes (1 + (20/10))",
			20,
			3,
		},
		{
			"11 replicas for 100 nodes (1 + (100/10) )",
			100,
			11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicaCount(tt.nodes); got != tt.want {
				t.Errorf("replicaCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
