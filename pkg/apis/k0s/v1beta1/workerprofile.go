// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
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
			errors = append(errors, err...)
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

// Validate validates instance
func (wp *WorkerProfile) Validate() []error {

	var errs []error

	kubeletCfg := &kubeletv1beta1.KubeletConfiguration{}
	if err := json.Unmarshal(wp.Config.Raw, kubeletCfg); err != nil {
		errs = append(errs, fmt.Errorf("failed to decode worker profile %q: %w", wp.Name, err))
	}
	if kubeletCfg.ClusterDNS != nil {
		errs = append(errs, fmt.Errorf("field `clusterDNS` is prohibited to override in worker profile %q", wp.Name))
	}
	if kubeletCfg.ClusterDomain != "" {
		errs = append(errs, fmt.Errorf("field `clusterDomain` is prohibited to override in worker profile %q", wp.Name))
	}
	if kubeletCfg.APIVersion != "" {
		errs = append(errs, fmt.Errorf("field `apiVersion` is prohibited to override in worker profile %q", wp.Name))
	}
	if kubeletCfg.Kind != "" {
		errs = append(errs, fmt.Errorf("field `kind` is prohibited to override in worker profile %q", wp.Name))
	}
	if kubeletCfg.StaticPodURL != "" {
		errs = append(errs, fmt.Errorf("field `staticPodURL` is prohibited to override in worker profile %q", wp.Name))
	}
	return errs
}
