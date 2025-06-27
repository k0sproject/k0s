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
}

func TestApiSuite(t *testing.T) {
	apiSuite := &APISuite{}

	suite.Run(t, apiSuite)
}
