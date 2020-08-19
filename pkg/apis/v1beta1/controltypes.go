package v1beta1

// CaResponse defines the reponse type for /ca control API
type CaResponse struct {
	Key  []byte `json:"key"`
	Cert []byte `json:"cert"`
}
