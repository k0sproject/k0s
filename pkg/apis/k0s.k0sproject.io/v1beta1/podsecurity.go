/*
Copyright 2022 k0s authors

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

	"github.com/k0sproject/k0s/pkg/constant"
)

var _ Validateable = (*PodSecurity)(nil)

var allowedStandards = map[string]struct{}{
	"":                                     {}, // allow empty value
	constant.PodSecurityStandardPrivileged: {},
	constant.PodSecurityStandardBaseline:   {},
	constant.PodSecurityStandardRestricted: {},
}

// PodSecurity defines the config options for setting default PodSecurity standards for namespaces
type PodSecurity struct {
	// default policy for enforce mode
	Enforce string `json:"enforce"`
	// default policy for audit mode
	Audit string `json:"audit"`
	// default policy for warn mode
	Warn string `json:"warn"`
}

// DefaultPodSecurity creates new PodSecurity with sane defaults
func DefaultPodSecurity() *PodSecurity {
	return &PodSecurity{}
}

func (p *PodSecurity) Enabled() bool {
	if p == nil {
		return false
	}
	return p.Enforce != "" || p.Audit != "" || p.Warn != ""
}

// Validate check that the given psp is one of the built-in ones
func (p *PodSecurity) Validate() []error {
	var errs []error
	if _, ok := allowedStandards[p.Enforce]; !ok {
		errs = append(errs, fmt.Errorf("%s is not a supported pod security standard", p.Enforce))
	}
	if _, ok := allowedStandards[p.Audit]; !ok {
		errs = append(errs, fmt.Errorf("%s is not a supported pod security standard", p.Audit))
	}
	if _, ok := allowedStandards[p.Warn]; !ok {
		errs = append(errs, fmt.Errorf("%s is not a supported pod security standard", p.Warn))
	}

	return errs
}
