// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerProfiles(t *testing.T) {
	t.Run("worker_profile_validation", func(t *testing.T) {
		cases := []struct {
			name string
			spec map[string]any
			msg  string
			bad  string
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
				msg: `workerProfiles[0].values.apiVersion: Invalid value: "v2": expected "kubelet.config.k8s.io/v1beta1"`,
				bad: "v2",
			},
			{
				name: "Locked field kind",
				spec: map[string]any{
					"kind": "Controller",
				},
				msg: `workerProfiles[0].values.kind: Invalid value: "Controller": expected "KubeletConfiguration"`,
				bad: "Controller",
			},
			{
				name: "Locked field clusterDNS",
				spec: map[string]any{
					"clusterDNS": []string{"8.8.8.8"},
				},
				msg: "workerProfiles[0].values.clusterDNS: Forbidden: may not be used in k0s worker profiles",
			},
			{
				name: "Locked field clusterDomain",
				spec: map[string]any{
					"clusterDomain": "cluster.org",
				},
				msg: "workerProfiles[0].values.clusterDomain: Forbidden: may not be used in k0s worker profiles",
			},
			{
				name: "Locked field staticPodURL",
				spec: map[string]any{
					"staticPodURL": "foo",
				},
				msg: "workerProfiles[0].values.staticPodURL: Forbidden: may not be used in k0s worker profiles",
			},
			{
				name: "Valid kubelet configuration",
				spec: map[string]any{
					"cpuManagerPolicy": "static",
					"cpuManagerPolicyOptions": map[string]string{
						"full-pcpus-only": "true",
					}},
			},
			{
				name: "Invalid kubelet configuration",
				spec: map[string]any{
					"cpuManagerPolicyOptions": "full-pcpus-only=true",
				},
				msg: "workerProfiles[0].values: Invalid value: json: cannot unmarshal string into Go struct field KubeletConfiguration.cpuManagerPolicyOptions of type map[string]string",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				value, err := json.Marshal(tc.spec)
				require.NoError(t, err)

				profiles := WorkerProfiles{{
					Config: &runtime.RawExtension{Raw: value},
				}}

				errs := profiles.Validate(field.NewPath("workerProfiles"))
				if tc.msg == "" {
					assert.Nilf(t, errs, "%s", errors.Join(errs...))
				} else if assert.Lenf(t, errs, 1, "%s", errors.Join(errs...)) {
					assert.Equal(t, tc.msg, errs[0].Error())

					var fieldErr *field.Error
					if assert.ErrorAs(t, errs[0], &fieldErr) {
						bad := any(tc.bad)
						if fieldErr.Field == "workerProfiles[0].values" {
							bad = value
						}
						assert.Equal(t, bad, fieldErr.BadValue)
					}
				}
			})
		}
	})
}
