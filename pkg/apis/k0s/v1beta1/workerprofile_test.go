// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerProfile worker profile test suite
func TestWorkerProfile(t *testing.T) {
	t.Run("worker_profile_validation", func(t *testing.T) {
		cases := []struct {
			name string
			spec map[string]any
			msg  string
		}{
			{
				name: "Generic spec is valid",
				spec: map[string]any{},
			},
			{
				name: "Locked field apiVersion",
				spec: map[string]any{
					"apiVersion": "v2",
				},
				msg: "field `apiVersion` is prohibited to override in worker profile \"Locked field apiVersion\"",
			},
			{
				name: "Locked field kind",
				spec: map[string]any{
					"kind": "Controller",
				},
				msg: "field `kind` is prohibited to override in worker profile \"Locked field kind\"",
			},
			{
				name: "Locked field clusterDNS",
				spec: map[string]any{
					"clusterDNS": []string{"8.8.8.8"},
				},
				msg: "field `clusterDNS` is prohibited to override in worker profile \"Locked field clusterDNS\"",
			},
			{
				name: "Locked field clusterDomain",
				spec: map[string]any{
					"clusterDomain": "cluster.org",
				},
				msg: "field `clusterDomain` is prohibited to override in worker profile \"Locked field clusterDomain\"",
			},
			{
				name: "Locked field staticPodURL",
				spec: map[string]any{
					"staticPodURL": "foo",
				},
				msg: "field `staticPodURL` is prohibited to override in worker profile \"Locked field staticPodURL\"",
			}, {
				name: "Valid kubelet configuration",
				spec: map[string]any{
					"cpuManagerPolicy": "static",
					"cpuManagerPolicyOptions": map[string]string{
						"full-pcpus-only": "true",
					}},
			}, {
				name: "Invalid kubelet configuration",
				spec: map[string]any{
					"cpuManagerPolicyOptions": "full-pcpus-only=true",
				},
				msg: "failed to decode worker profile \"Invalid kubelet configuration\": " +
					"json: cannot unmarshal string into Go struct field KubeletConfiguration.cpuManagerPolicyOptions of type map[string]string",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := json.Marshal(tc.spec)
				require.NoError(t, err)

				profile := WorkerProfile{
					Name:   tc.name,
					Config: &runtime.RawExtension{Raw: value},
				}

				errs := profile.Validate()
				if tc.msg == "" {
					assert.Nilf(t, errs, "%s", errors.Join(errs...))
				} else if assert.Lenf(t, errs, 1, "%s", errors.Join(errs...)) {
					assert.Equal(t, tc.msg, errs[0].Error())
				}
			})
		}
	})
}
