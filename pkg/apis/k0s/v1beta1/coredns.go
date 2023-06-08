/*
Copyright 2023 k0s authors

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

// CoreDNSSpec defines CoreDNS config options
type CoreDNSSpec struct {
	// Disable prometheus.io/scrape annotation (default: false)
	DisablePrometheusScrapeAnnotation bool `json:"disablePrometheusScrapeAnnotation,omitempty"`
}

// DefaultCoreDNSSpec creates CoreDNSSpec with sane defaults
func DefaultCoreDNSSpec() *CoreDNSSpec {
	return &CoreDNSSpec{
		DisablePrometheusScrapeAnnotation: false,
	}
}

// Validate validates CoreDNS specs correctness
func (c *CoreDNSSpec) Validate() []error {
	if c == nil {
		return nil
	}

	return nil
}
