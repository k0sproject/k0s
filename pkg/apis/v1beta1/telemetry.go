package v1beta1

import "time"

// ClusterTelemetry holds telemetry related settings
type ClusterTelemetry struct {
	Interval time.Duration `yaml:"interval"`
	Enabled  bool          `yaml:"enabled"`
}

// DefaultClusterTelemetry default settings
func DefaultClusterTelemetry() *ClusterTelemetry {
	return &ClusterTelemetry{
		Interval: time.Second * 120,
		Enabled:  true,
	}
}
