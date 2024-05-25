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
}

func TestApiSuite(t *testing.T) {
	apiSuite := &APISuite{}

	suite.Run(t, apiSuite)
}
