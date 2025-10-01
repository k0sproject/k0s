// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
)

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
		return errors.New("node cannot be empty")
	}

	if e.PeerAddress == "" {
		return errors.New("peerAddress cannot be empty")
	}

	return nil
}

// EtcdResponse defines the etcd control api response structure
type EtcdResponse struct {
	CA             CaResponse `json:"ca"`
	InitialCluster []string   `json:"initialCluster"`
}
