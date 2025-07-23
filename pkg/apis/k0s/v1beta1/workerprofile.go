// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

var _ Validateable = (*WorkerProfiles)(nil)

// WorkerProfiles profiles collection
// +listType=map
// +listMapKey=name
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
type WorkerProfile struct {
	// String; name to use as profile selector for the worker process
	Name string `json:"name"`
	// Worker Mapping object
	// +kubebuilder:validation:type=object
	Config *runtime.RawExtension `json:"values,omitempty"`
}

var lockedFields = map[string]struct{}{
	"clusterDNS":    {},
	"clusterDomain": {},
	"apiVersion":    {},
	"kind":          {},
	"staticPodURL":  {},
}

// Validate validates instance
func (wp *WorkerProfile) Validate() error {
	var parsed map[string]any
	err := json.Unmarshal(wp.Config.Raw, &parsed)
	if err != nil {
		return err
	}

	for field := range parsed {
		if _, found := lockedFields[field]; found {
			return fmt.Errorf("field `%s` is prohibited to override in worker profile", field)
		}
	}
	return nil
}
