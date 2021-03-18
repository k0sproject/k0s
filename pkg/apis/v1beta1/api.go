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

	"github.com/k0sproject/k0s/internal/util"

	"github.com/asaskevich/govalidator"
)

// APISpec ...
type APISpec struct {
	Address         string            `yaml:"address"`
	Port            int               `yaml:"port"`
	K0sApiPort      int               `yaml:"k0s_api_port,omitempty"`
	ExternalAddress string            `yaml:"externalAddress,omitempty"`
	SANs            []string          `yaml:"sans"`
	ExtraArgs       map[string]string `yaml:"extraArgs,omitempty"`
}

// DefaultAPISpec default settings for api
func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := util.AllAddresses()
	publicAddress, _ := util.FirstPublicAddress()
	return &APISpec{
		Port:       6443,
		K0sApiPort: 9443,
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
	return a.getExternalURIForPort(a.K0sApiPort)
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
	sans, _ := util.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalAddress)
	}

	return util.Unique(sans)
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

	return errors
}
