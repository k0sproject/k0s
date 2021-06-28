package v1beta1

// var _ Validateable = (*ClusterTelemetry)(nil)

// ClusterTelemetry holds telemetry related settings
type ClusterTelemetry struct {
	Enabled bool `json:"enabled"`
}

// DefaultClusterTelemetry default settings
func DefaultClusterTelemetry() *ClusterTelemetry {
	return &ClusterTelemetry{
		Enabled: true,
	}
}

// Validate stub for Validateable interface
func (c *ClusterTelemetry) Validate() []error {
	return nil
}
