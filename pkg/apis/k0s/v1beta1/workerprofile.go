// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"
)

// WorkerProfiles profiles collection
// +listType=map
// +listMapKey=name
type WorkerProfiles []WorkerProfile

// Validate validates all profiles
func (wps WorkerProfiles) Validate(path *field.Path) []error {
	var errors []error
	for i, p := range wps {
		if err := p.Validate(path.Index(i)); err != nil {
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
func (wp *WorkerProfile) Validate(path *field.Path) (errs []error) {
	// The name is used as part of a ConfigMap name and used as a label value.
	// Validate it accordingly.
	if wp.Name == "" {
		errs = append(errs, field.Required(path.Child("name"), ""))
	} else if msgs := validation.IsDNS1123Label(wp.Name); len(msgs) > 0 {
		errs = append(errs, field.Invalid(path.Child("name"), wp.Name, strings.Join(msgs, ", ")))
	}

	if wp.Config != nil {
		errs = append(errs, wp.validateConfig(path.Child("values"))...)
	}

	return errs
}

func (wp *WorkerProfile) validateConfig(path *field.Path) []error {
	var errs []error

	// Decode the kubelet config.
	kubeletCfg := &kubeletv1beta1.KubeletConfiguration{}
	if err := json.Unmarshal(wp.Config.Raw, kubeletCfg); err != nil {
		errs = append(errs, (*shortenedFieldError)(field.Invalid(path, wp.Config.Raw, err.Error())))
		return errs
	}

	// Check that apiVersion and kind are either unspecified or match the expected values.
	if kubeletCfg.APIVersion != "" && kubeletCfg.APIVersion != kubeletv1beta1.SchemeGroupVersion.String() {
		detail := fmt.Sprintf("expected %q", kubeletv1beta1.SchemeGroupVersion)
		errs = append(errs, field.Invalid(path.Child("apiVersion"), kubeletCfg.APIVersion, detail))
	}
	if kubeletCfg.Kind != "" && kubeletCfg.Kind != "KubeletConfiguration" {
		detail := fmt.Sprintf("expected %q", "KubeletConfiguration")
		errs = append(errs, field.Invalid(path.Child("kind"), kubeletCfg.Kind, detail))
	}

	// Check that k0s-reserved config flags remain untouched.
	reservedField := func(name string) *field.Error {
		return field.Forbidden(path.Child(name), "may not be used in k0s worker profiles")
	}
	if kubeletCfg.ClusterDNS != nil {
		errs = append(errs, reservedField("clusterDNS"))
	}
	if kubeletCfg.ClusterDomain != "" {
		errs = append(errs, reservedField("clusterDomain"))
	}
	if kubeletCfg.StaticPodURL != "" {
		errs = append(errs, reservedField("staticPodURL"))
	}

	return errs
}

// A [field.Error] that won't include the bad value in its error message.
type shortenedFieldError field.Error

func (e *shortenedFieldError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Field, e.Type, e.Detail)
}

func (e *shortenedFieldError) Unwrap() error {
	return (*field.Error)(e)
}
