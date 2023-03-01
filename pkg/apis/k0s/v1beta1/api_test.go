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
	"testing"

	"github.com/stretchr/testify/suite"
)

type APISuite struct {
	suite.Suite
}

func (s *APISuite) TestValidation() {
	s.T().Run("defaults_are_valid", func(t *testing.T) {
		a := DefaultAPISpec()

		s.Nil(a.Validate())
	})

	s.T().Run("accepts_ipv6_as_address", func(t *testing.T) {
		a := APISpec{
			Address: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		}

		s.Nil(a.Validate())

	})

	s.T().Run("invalid_api_address", func(t *testing.T) {
		a := APISpec{
			Address: "something.that.is.not.valid//(())",
		}

		errors := a.Validate()
		s.NotNil(errors)
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `address: Invalid value: "something.that.is.not.valid//(())": invalid IP address`)
		}
	})

	s.T().Run("invalid_sans_address", func(t *testing.T) {
		a := APISpec{
			Address: "1.2.3.4",
			SANs: []string{
				"something.that.is.not.valid//(())",
			},
		}

		errors := a.Validate()
		s.NotNil(errors)
		if s.Len(errors, 1) {
			s.ErrorContains(errors[0], `sans[0]: Invalid value: "something.that.is.not.valid//(())": invalid IP address / DNS name`)
		}
	})
	s.T().Run("TunneledNetworkingMode_and_default_kas_port_is_invalid", func(t *testing.T) {
		a := DefaultAPISpec()
		a.TunneledNetworkingMode = true
		errors := a.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "can't use default kubeapi port if TunneledNetworkingMode is enabled")
	})
}

func TestApiSuite(t *testing.T) {
	apiSuite := &APISuite{}

	suite.Run(t, apiSuite)
}
