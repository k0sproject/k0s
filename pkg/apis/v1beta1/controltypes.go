package v1beta1

import "fmt"

// CaResponse defines the reponse type for /ca control API
type CaResponse struct {
	Key  []byte `json:"key"`
	Cert []byte `json:"cert"`
}

// EtcdRequest defines the etcd control api request structure
type EtcdRequest struct {
	Node        string `json:"node"`
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
