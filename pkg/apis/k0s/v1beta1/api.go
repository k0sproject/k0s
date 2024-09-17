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
	"encoding/json"
	"fmt"
	"net"

	"github.com/k0sproject/k0s/internal/pkg/iface"
	"github.com/k0sproject/k0s/internal/pkg/stringslice"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/asaskevich/govalidator"
)

var _ Validateable = (*APISpec)(nil)

// APISpec defines the settings for the K0s API
type APISpec struct {
	// Address on which to connect to the API server.
	Address string `json:"address,omitempty"`

	// Whether to only bind to the IP given by the address option.
	// +optional
	OnlyBindToAddress bool `json:"onlyBindToAddress,omitempty"`

	// The loadbalancer address (for k0s controllers running behind a loadbalancer)
	ExternalAddress string `json:"externalAddress,omitempty"`

	// Map of key-values (strings) for any extra arguments to pass down to Kubernetes api-server process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// Custom port for k0s-api server to listen on (default: 9443)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9443
	K0sAPIPort int `json:"k0sApiPort,omitempty"`

	// Custom port for kube-api server to listen on (default: 6443)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=6443
	Port int `json:"port,omitempty"`

	// List of additional addresses to push to API servers serving the certificate
	SANs []string `json:"sans,omitempty"`
}

// DefaultAPISpec default settings for api
func DefaultAPISpec() *APISpec {
	a := new(APISpec)
	a.setDefaults()
	a.SANs, _ = iface.AllAddresses()
	return a
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

// Sans return the given SANS plus all local addresses and externalAddress if given
func (a *APISpec) Sans() []string {
	sans, _ := iface.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalAddress)
	}

	return stringslice.Unique(sans)
}

func isAnyAddress(address string) bool {
	return address == "0.0.0.0" || address == "::"
}

// Validate validates APISpec struct
func (a *APISpec) Validate() []error {
	if a == nil {
		return nil
	}

	var errors []error

	if !govalidator.IsIP(a.Address) {
		errors = append(errors, field.Invalid(field.NewPath("address"), a.Address, "invalid IP address"))
	}
	if isAnyAddress(a.Address) {
		errors = append(errors, field.Invalid(field.NewPath("address"), a.Address, "invalid INADDR_ANY"))
	}

	validateIPAddressOrDNSName := func(path *field.Path, san string) {
		if govalidator.IsIP(san) || govalidator.IsDNSName(san) {
			return
		}
		errors = append(errors, field.Invalid(path, san, "invalid IP address / DNS name"))
	}

	if a.ExternalAddress != "" {
		validateIPAddressOrDNSName(field.NewPath("externalAddress"), a.ExternalAddress)
		if isAnyAddress(a.ExternalAddress) {
			errors = append(errors, field.Invalid(field.NewPath("externalAddress"), a.Address, "invalid INADDR_ANY"))
		}
	}

	for _, msg := range validation.IsValidPortNum(a.K0sAPIPort) {
		errors = append(errors, field.Invalid(field.NewPath("k0sApiPort"), a.K0sAPIPort, msg))
	}

	for _, msg := range validation.IsValidPortNum(a.Port) {
		errors = append(errors, field.Invalid(field.NewPath("port"), a.Port, msg))
	}

	sansPath := field.NewPath("sans")
	for idx, san := range a.SANs {
		validateIPAddressOrDNSName(sansPath.Index(idx), san)
	}

	return errors
}

// Sets in some sane defaults when unmarshaling the data from JSON.
func (a *APISpec) UnmarshalJSON(data []byte) error {
	type apiSpec APISpec
	jc := (*apiSpec)(a)

	if err := json.Unmarshal(data, jc); err != nil {
		return err
	}

	a.setDefaults()
	return nil
}

func (a *APISpec) setDefaults() {
	if a.Address == "" {
		a.Address, _ = iface.FirstPublicAddress()
	}
	if a.K0sAPIPort == 0 {
		a.K0sAPIPort = 9443
	}
	if a.Port == 0 {
		a.Port = 6443
	}
}
