package v1beta1

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

// TestWorkerProfile worker profile test suite
func TestWorkerProfile(t *testing.T) {
	t.Run("worker_profile_validation", func(t *testing.T) {
		cases := []struct {
			name  string
			spec  map[string]string
			valid bool
		}{
			{
				name:  "Generic spec is valid",
				spec:  map[string]string{},
				valid: true,
			},
			{
				name: "Locked field clusterDNS",
				spec: map[string]string{
					"clusterDNS": "8.8.8.8",
				},
				valid: false,
			},
			{
				name: "Locked field clusterDomain",
				spec: map[string]string{
					"clusterDomain": "cluster.org",
				},
				valid: false,
			},
			{
				name: "Locked field apiVersion",
				spec: map[string]string{
					"apiVersion": "v2",
				},
				valid: false,
			},
			{
				name: "Locked field kind",
				spec: map[string]string{
					"kind": "Controller",
				},
				valid: false,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				profile := WorkerProfile{
					Values: tc.spec,
				}
				valid := profile.Validate() == nil
				assert.Equal(t, valid, tc.valid)
			})
		}
	})
}
