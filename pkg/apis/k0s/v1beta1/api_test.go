// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type APISuite struct {
	suite.Suite
}

func (s *APISuite) TestValidation() {
	s.Run("defaults_are_valid", func() {
		a := DefaultAPISpec()

		s.NoError(errors.Join(a.Validate()...))
	})

	s.Run("accepts_ipv6_as_address", func() {
		ipV6Addr := "2001:0db8:85a3:0000:0000:8a2e:0370:7334"
		a := APISpec{Address: ipV6Addr}
		a.setDefaults()

		s.Equal(ipV6Addr, a.Address)
		s.NoError(errors.Join(a.Validate()...))
	})

	s.Run("invalid_api_address", func() {
		a := APISpec{
			Address: "something.that.is.not.valid//(())",
		}
		a.setDefaults()

		errors := a.Validate()
		s.NotNil(errors)
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `address: Invalid value: "something.that.is.not.valid//(())": invalid IP address`)
		}
	})

	s.Run("invalid_sans_address", func() {
		a := APISpec{
			SANs: []string{
				"something.that.is.not.valid//(())",
			},
		}
		a.setDefaults()

		errors := a.Validate()
		s.NotNil(errors)
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `sans[0]: Invalid value: "something.that.is.not.valid//(())": invalid IP address / DNS name`)
		}
	})

	s.Run("rejects any address as external address", func() {
		for _, externalAddress := range []string{"0.0.0.0", "0.0.0.0:6443", "::", "[::]:6443"} {
			s.Run(externalAddress, func() {
				a := APISpec{Address: "192.168.1.10", ExternalAddress: externalAddress}
				a.setDefaults()

				errors := a.Validate()
				if s.Len(errors, 1) {
					s.ErrorContains(errors[0], `externalAddress: Invalid value: "`+externalAddress+`": invalid INADDR_ANY`)
				}
			})
		}
	})
}

func (s *APISuite) TestExternalAddressURLs() {
	for _, test := range []struct {
		name            string
		externalAddress string
		apiServerURL    string
		k0sAPIURL       string
	}{
		{"without external address", "", "https://192.168.1.10:6443", "https://192.168.1.10:9443"},
		{"with host", "lb.example.com", "https://lb.example.com:6443", "https://lb.example.com:9443"},
		{"with host and port", "lb.example.com:443", "https://lb.example.com:443", "https://lb.example.com:9443"},
		{"with IPv4 address and port", "10.0.0.1:443", "https://10.0.0.1:443", "https://10.0.0.1:9443"},
		{"with IPv6 address", "2001:db8::1", "https://[2001:db8::1]:6443", "https://[2001:db8::1]:9443"},
		{"with IPv6 address and port", "[2001:db8::1]:443", "https://[2001:db8::1]:443", "https://[2001:db8::1]:9443"},
	} {
		s.Run(test.name, func() {
			a := APISpec{Address: "192.168.1.10", ExternalAddress: test.externalAddress}
			a.setDefaults()

			s.Equal(test.apiServerURL, a.APIAddressURL(), "API server URL")
			s.Equal(test.k0sAPIURL, a.K0sControlPlaneAPIAddress(), "k0s Join API URL")
		})
	}
}

func TestApiSuite(t *testing.T) {
	apiSuite := &APISuite{}

	suite.Run(t, apiSuite)
}
