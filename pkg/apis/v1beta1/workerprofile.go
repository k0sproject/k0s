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
)

var _ Validateable = (*WorkerProfiles)(nil)

// WorkerProfiles profiles collection
type WorkerProfiles []WorkerProfile

// Validate validates all profiles
func (wps WorkerProfiles) Validate() []error {
	var errors []error
	for _, p := range wps {
		if err := p.Validate(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// WorkerProfile worker profile
// +k8s:deepcopy-gen=false
type WorkerProfile struct {
	// String; name to use as profile selector for the worker process
	Name string `json:"name" yaml:"name"`
	// Worker Mapping object
	Values `json:"values" yaml:"values"`
}

// +k8s:deepcopy-gen=false
type Values map[string]interface{}

var lockedFields = map[string]struct{}{
	"clusterDNS":    {},
	"clusterDomain": {},
	"apiVersion":    {},
	"kind":          {},
}

// Validate validates instance
func (wp *WorkerProfile) Validate() error {
	for field := range wp.Values {
		if _, found := lockedFields[field]; found {
			return fmt.Errorf("field `%s` is prohibited to override in worker profile", field)
		}
	}
	return nil
}
