// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"bytes"
	"encoding/json"
)

var _ Validateable = (*ClusterTelemetry)(nil)

// ClusterTelemetry holds telemetry related settings
type ClusterTelemetry struct {
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
}

// UnmarshalJSON decodes ClusterTelemetry while tolerating the removed
// "interval" field. It used to configure the telemetry interval and may
// still be present in on-disk configs (e.g. written by k0sctl), so it's
// accepted and discarded here instead of failing strict decoding.
func (t *ClusterTelemetry) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	type clusterTelemetry ClusterTelemetry
	telemetry := struct {
		*clusterTelemetry
		Interval json.RawMessage `json:"interval"`
	}{(*clusterTelemetry)(t), nil}

	return decoder.Decode(&telemetry)
}

func (t *ClusterTelemetry) IsEnabled() bool {
	return t != nil && t.Enabled != nil && *t.Enabled
}

// DefaultClusterTelemetry default settings
func DefaultClusterTelemetry() *ClusterTelemetry {
	return &ClusterTelemetry{
		Enabled: new(false),
	}
}

// Validate stub for Validateable interface
func (c *ClusterTelemetry) Validate() []error {
	return nil
}
