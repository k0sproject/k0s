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

package v1beta1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestWorkerProfile worker profile test suite
func TestWorkerProfile(t *testing.T) {
	t.Run("worker_profile_validation", func(t *testing.T) {
		cases := []struct {
			name  string
			spec  map[string]interface{}
			valid bool
		}{
			{
				name:  "Generic spec is valid",
				spec:  map[string]interface{}{},
				valid: true,
			},
			{
				name: "Locked field clusterDNS",
				spec: map[string]interface{}{
					"clusterDNS": "8.8.8.8",
				},
				valid: false,
			},
			{
				name: "Locked field clusterDomain",
				spec: map[string]interface{}{
					"clusterDomain": "cluster.org",
				},
				valid: false,
			},
			{
				name: "Locked field apiVersion",
				spec: map[string]interface{}{
					"apiVersion": "v2",
				},
				valid: false,
			},
			{
				name: "Locked field kind",
				spec: map[string]interface{}{
					"kind": "Controller",
				},
				valid: false,
			}, {
				name: "Locked field staticPodURL",
				spec: map[string]interface{}{
					"staticPodURL": "foo",
				},
				valid: false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := json.Marshal(tc.spec)
				if err != nil {
					t.Fatal(err)
				}
				profile := WorkerProfile{Config: &runtime.RawExtension{Raw: value}}
				valid := profile.Validate() == nil
				assert.Equal(t, valid, tc.valid)
			})
		}
	})
}
