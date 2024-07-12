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

import "fmt"

// CaResponse defines the response type for /ca control API
type CaResponse struct {
	Key   []byte `json:"key"`
	Cert  []byte `json:"cert"`
	SAKey []byte `json:"saKey"`
	SAPub []byte `json:"saPub"`
}

// EtcdRequest defines the etcd control api request structure
type EtcdRequest struct {
	// +kubebuilder:validation:MinLength=1
	Node string `json:"node"`
	// +kubebuilder:validation:MinLength=1
	PeerAddress string `json:"peerAddress"`
}

// Validate validates the request
func (e *EtcdRequest) Validate() error {
	if e.Node == "" {
		return fmt.Errorf("node cannot be empty")
	}

	if e.PeerAddress == "" {
		return fmt.Errorf("peerAddress cannot be empty")
	}

	return nil
}

// EtcdResponse defines the etcd control api response structure
type EtcdResponse struct {
	CA             CaResponse `json:"ca"`
	InitialCluster []string   `json:"initialCluster"`
}
