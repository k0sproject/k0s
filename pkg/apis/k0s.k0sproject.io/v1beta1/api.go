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

import (
	"fmt"
	"net"

	"github.com/asaskevich/govalidator"
	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"
)

var _ Validateable = (*APISpec)(nil)

// APISpec defines the settings for the K0s API
type APISpec struct {
	// Local address on which to bind an API
	Address string `json:"address,omitempty"`

	// The loadbalancer address (for k0s controllers running behind a loadbalancer)
	ExternalAddress string `json:"externalAddress,omitempty"`

	// Map of key-values (strings) for any extra arguments to pass down to Kubernetes api-server process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// Custom port for k0s-api server to listen on (default: 9443)
	K0sAPIPort int `json:"k0sApiPort,omitempty"`

	// Custom port for kube-api server to listen on (default: 6443)
	Port int `json:"port,omitempty"`

	// List of additional addresses to push to API servers serving the certificate
	SANs []string `json:"sans,omitempty"`
}

// DefaultAPISpec default settings for api
func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := iface.AllAddresses()
	publicAddress, _ := iface.FirstPublicAddress()
	return &APISpec{
		Port:       6443,
		K0sAPIPort: 9443,
		SANs:       addresses,
		Address:    publicAddress,
		ExtraArgs:  make(map[string]string),
	}
}

// APIAddress ...
func (a *APISpec) APIAddress() string {
	if a.ExternalAddress != "" {
		return a.ExternalAddress
	}
	return a.Address
}

// APIAddressURL returns kube-apiserver external URI
func (a *APISpec) APIAddressURL() string {
	return a.getExternalURIForPort(a.Port)
}

// IsIPv6String returns if ip is IPv6.
func IsIPv6String(ip string) bool {
	netIP := net.ParseIP(ip)
	return netIP != nil && netIP.To4() == nil
}

// K0sControlPlaneAPIAddress returns the controller join APIs address
func (a *APISpec) K0sControlPlaneAPIAddress() string {
	return a.getExternalURIForPort(a.K0sAPIPort)
}

func (a *APISpec) getExternalURIForPort(port int) string {
	addr := a.Address
	if a.ExternalAddress != "" {
		addr = a.ExternalAddress
	}
	if IsIPv6String(addr) {
		return fmt.Sprintf("https://[%s]:%d", addr, port)
	}
	return fmt.Sprintf("https://%s:%d", addr, port)
}

// Sans return the given SANS plus all local adresses and externalAddress if given
func (a *APISpec) Sans() []string {
	sans, _ := iface.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalAddress)
	}

	return stringslice.Unique(sans)
}

// Validate validates APISpec struct
func (a *APISpec) Validate() []error {
	var errors []error

	for _, a := range a.Sans() {
		if govalidator.IsIP(a) {
			continue
		}
		if govalidator.IsDNSName(a) {
			continue
		}
		errors = append(errors, fmt.Errorf("%s is not a valid address for sans", a))
	}

	if !govalidator.IsIP(a.Address) {
		errors = append(errors, fmt.Errorf("spec.api.address: %q is not IP address", a.Address))
	}

	return errors
}
