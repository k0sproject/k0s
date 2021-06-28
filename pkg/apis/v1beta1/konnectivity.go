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

package v1beta1

// var _ Validateable = (*KonnectivitySpec)(nil)

// KonnectivitySpec defines the requested state for Konnectivity
type KonnectivitySpec struct {
	// agent port to listen on (default 8132)
	AgentPort int64 `json:"agentPort,omitempty"`
	// admin port to listen on (default 8133)
	AdminPort int64 `json:"adminPort,omitempty"`
}

// DefaultKonnectivitySpec builds default KonnectivitySpec
func DefaultKonnectivitySpec() *KonnectivitySpec {
	return &KonnectivitySpec{
		AdminPort: 8133,
		AgentPort: 8132,
	}
}

// Validate stub for Validateable interface
func (k *KonnectivitySpec) Validate() []error {
	return nil
}
