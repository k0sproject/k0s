// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/k0sproject/k0s/internal/pkg/iface"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
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
	OnlyBindToAddress bool `json:"onlyBindToAddress,omitempty"`

	// The loadbalancer address (for k0s controllers running behind a loadbalancer)
	ExternalAddress string `json:"externalAddress,omitempty"`

	// Map of key-values (strings) for any extra arguments to pass down to Kubernetes api-server process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// Slice of strings with raw arguments to pass to the kube-apiserver process
	// These arguments will be appended to the `ExtraArgs` and aren't validated at all.
	// ExtraArgs are recommended over RawArgs. If possible use ExtraArgs to set arguments.
	RawArgs []string `json:"rawArgs,omitempty"`

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
	// +listType=set
	SANs []string `json:"sans,omitempty"`

	// Custom config for CA certificates.
	CA *CA `json:"ca,omitempty"`
}

// DefaultAPISpec default settings for api
func DefaultAPISpec() *APISpec {
	a := new(APISpec)
	a.setDefaults()
	return a
}

func (a *APISpec) LocalURL() *url.URL {
	var host string
	if a.OnlyBindToAddress {
		host = net.JoinHostPort(a.Address, strconv.Itoa(a.Port))
	} else {
		host = fmt.Sprintf("localhost:%d", a.Port)
	}

	return &url.URL{Scheme: "https", Host: host}
}

func (a *APISpec) APIServerHostPort() (*k0snet.HostPort, error) {
	if a.ExternalAddress != "" {
		if ip := net.ParseIP(a.ExternalAddress); ip != nil {
			return k0snet.NewHostPort(a.ExternalAddress, uint16(a.Port))
		}
		hostPort, err := k0snet.ParseHostPortWithDefault(a.ExternalAddress, uint16(a.Port))
		if err != nil {
			return nil, fmt.Errorf("external address is invalid: %w", err)
		}
		return hostPort, nil
	}

	return k0snet.NewHostPort(a.Address, uint16(a.Port))
}

func (a *APISpec) ExternalHost() string {
	if a.ExternalAddress != "" {
		host, _, _ := net.SplitHostPort(a.ExternalAddress)
		if host != "" {
			return host
		}
	}
	return a.ExternalAddress
}

func (a *APISpec) ExternalPort() int {
	if a.ExternalAddress != "" {
		_, port, _ := net.SplitHostPort(a.ExternalAddress)
		if portInt, err := strconv.Atoi(port); port != "" && err == nil {
			return portInt
		}
	}
	return a.Port
}

// APIAddressURL returns kube-apiserver external URI
func (a *APISpec) APIAddressURL() string {
	return a.getExternalURIForPort(a.Port)
}

// DetectPrimaryAddressFamily tries to detect the primary address of the cluster
// based on the address family of ExternalAddress. If this isn't set it will try
// to detect it based on the address family of Address.
// If the address used to detect it, isn't an IP address but a hostname or if
// both are unset, it will default to IPv4
func (a *APISpec) DetectPrimaryAddressFamily() PrimaryAddressFamilyType {
	if ip := net.ParseIP(cmp.Or(a.ExternalHost(), a.Address)); ip != nil && ip.To4() == nil {
		return PrimaryFamilyIPv6
	}
	return PrimaryFamilyIPv4
}

// K0sControlPlaneAPIAddress returns the controller join APIs address
func (a *APISpec) K0sControlPlaneAPIAddress() string {
	return a.getExternalURIForPort(a.K0sAPIPort)
}

func (a *APISpec) getExternalURIForPort(port int) string {
	addr := a.Address
	if a.ExternalAddress != "" {
		// If ExternalAddress is a full host:port address, return it as is
		if a.ExternalHost() != a.ExternalAddress {
			return (&url.URL{Scheme: "https", Host: a.ExternalAddress}).String()
		}

		addr = a.ExternalAddress
	}
	return (&url.URL{Scheme: "https", Host: net.JoinHostPort(addr, strconv.Itoa(port))}).String()
}

// Sans return the given SANS plus all local addresses and externalAddress if given
func (a *APISpec) Sans() []string {
	sans, _ := iface.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalHost())
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
		validateIPAddressOrDNSName(field.NewPath("externalAddress"), a.ExternalHost())
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
	if a.CA == nil {
		a.CA = DefaultCA()
	}
}
