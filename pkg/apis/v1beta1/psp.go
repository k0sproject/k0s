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

	"github.com/k0sproject/k0s/pkg/constant"
)

var _ Validateable = (*PodSecurityPolicy)(nil)

// PodSecurityPolicy defines the config options for setting system level default PSP
type PodSecurityPolicy struct {
	// default PSP for the cluster (00-k0s-privileged/99-k0s-restricted)
	DefaultPolicy string `json:"defaultPolicy" yaml:"defaultPolicy"`
}

// DefaultPodSecurityPolicy creates new PodSecurityPolicy with sane defaults
func DefaultPodSecurityPolicy() *PodSecurityPolicy {
	return &PodSecurityPolicy{
		DefaultPolicy: constant.DefaultPSP,
	}
}

// Validate check that the given psp is one of the built-in ones
func (p *PodSecurityPolicy) Validate() []error {
	if p.DefaultPolicy != "00-k0s-privileged" && p.DefaultPolicy != "99-k0s-restricted" {
		return []error{fmt.Errorf("%s is not a built-in pod security policy", p.DefaultPolicy)}
	}
	return nil
}
