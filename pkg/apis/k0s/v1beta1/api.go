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

	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/asaskevich/govalidator"
)

var _ Validateable = (*APISpec)(nil)

// APISpec defines the settings for the K0s API
type APISpec struct {
	// The local bind address for API servers.
	Address string `json:"address"`

	// The external loadbalancer address for k0s controllers.
	ExternalAddress string `json:"externalAddress,omitempty"`
	// Key-value pairs of additional arguments for kube-API server.
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// The listening port for k0s-API server (default: 9443).
	K0sAPIPort int `json:"k0sApiPort,omitempty"`

	// The listening port for kube-API server (default: 6443).
	Port int `json:"port"`

	// A list of additional SANs for the API server's TLS certificate.
	SANs []string `json:"sans"`
}

const defaultKasPort = 6443

// DefaultAPISpec default settings for api.
func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := iface.AllAddresses()
	publicAddress, _ := iface.FirstPublicAddress()
	return &APISpec{
		Port:       defaultKasPort,
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

// APIAddressURL returns kube-apiserver external URI.
func (a *APISpec) APIAddressURL() string {
	return a.getExternalURIForPort(a.Port)
}

// IsIPv6String returns if ip is IPv6.
func IsIPv6String(ip string) bool {
	netIP := net.ParseIP(ip)
	return netIP != nil && netIP.To4() == nil
}

// K0sControlPlaneAPIAddress returns the controller join APIs address.
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

// Sans return the given SANS plus all local adresses and externalAddress if given.
func (a *APISpec) Sans() []string {
	sans, _ := iface.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalAddress)
	}

	return stringslice.Unique(sans)
}

// Validate validates APISpec struct.
func (a *APISpec) Validate() []error {
	if a == nil {
		return nil
	}

	var errors []error

	if !govalidator.IsIP(a.Address) {
		errors = append(errors, field.Invalid(field.NewPath("address"), a.Address, "invalid IP address"))
	}

	validateIPAddressOrDNSName := func(path *field.Path, san string) {
		if govalidator.IsIP(san) || govalidator.IsDNSName(san) {
			return
		}
		errors = append(errors, field.Invalid(path, san, "invalid IP address / DNS name"))
	}

	sansPath := field.NewPath("sans")
	for idx, san := range a.SANs {
		validateIPAddressOrDNSName(sansPath.Index(idx), san)
	}

	if a.ExternalAddress != "" {
		validateIPAddressOrDNSName(field.NewPath("externalAddress"), a.ExternalAddress)
	}
	return errors
}
