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

type PSPSuite struct {
	suite.Suite
}

func (s *PSPSuite) TestValidation() {
	s.T().Run("defaults_are_valid", func(t *testing.T) {
		p := DefaultPodSecurityPolicy()

		s.Nil(p.Validate())
	})

	s.T().Run("restricted_psp", func(t *testing.T) {
		p := PodSecurityPolicy{
			DefaultPolicy: "99-k0s-restricted",
		}
		s.Nil(p.Validate())
	})

	s.T().Run("invalid_psp", func(t *testing.T) {
		p := PodSecurityPolicy{
			DefaultPolicy: "foobar",
		}

		errors := p.Validate()
		s.NotNil(errors)
		s.Len(errors, 1)
		s.Contains(errors[0].Error(), "is not a built-in pod security policy")
	})
}

func TestPSPSuite(t *testing.T) {
	pspSuite := &PSPSuite{}

	suite.Run(t, pspSuite)
}
