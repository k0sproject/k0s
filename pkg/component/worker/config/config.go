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

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"sigs.k8s.io/yaml"
)

type Profile struct {
	APIServerAddresses     []net.HostPort
	KubeletConfiguration   kubeletv1beta1.KubeletConfiguration
	NodeLocalLoadBalancing *v1beta1.NodeLocalLoadBalancing
	Konnectivity           Konnectivity
	PauseImage             *v1beta1.ImageSpec
	DualStackEnabled       bool
}

func (p *Profile) DeepCopy() *Profile {
	if p == nil {
		return nil
	}
	out := new(Profile)
	p.DeepCopyInto(out)
	return out
}

func (p *Profile) DeepCopyInto(out *Profile) {
	*out = *p

	if p.APIServerAddresses != nil {
		out.APIServerAddresses = slices.Clone(p.APIServerAddresses)
	}
	p.KubeletConfiguration.DeepCopyInto(&out.KubeletConfiguration)
	if p.NodeLocalLoadBalancing != nil {
		in, out := &p.NodeLocalLoadBalancing, &out.NodeLocalLoadBalancing
		*out = new(v1beta1.NodeLocalLoadBalancing)
		(*in).DeepCopyInto(*out)
	}
}

func (p *Profile) Validate(path *field.Path) (errs field.ErrorList) {
	if p == nil {
		return
	}

	errs = append(errs, p.NodeLocalLoadBalancing.Validate(path.Child("nodeLocalLoadBalancing"))...)
	errs = append(errs, p.Konnectivity.Validate(path.Child("konnectivity"))...)

	return
}

type Konnectivity struct {
	Enabled   bool   `json:"enabled,omitempty"`
	AgentPort uint16 `json:"agentPort,omitempty"`
}

func (k *Konnectivity) Validate(path *field.Path) (errs field.ErrorList) {
	if k == nil {
		return
	}

	agentPort := int(k.AgentPort)
	for _, msg := range validation.IsValidPortNum(agentPort) {
		errs = append(errs, field.Invalid(path.Child("agentPort"), agentPort, msg))
	}

	return
}

func FromConfigMapData(data map[string]string) (*Profile, error) {
	var config Profile
	var errs []error
	forEachConfigMapEntry(&config, func(fieldName string, ptr any) {
		data, ok := data[fieldName]
		if ok {
			if err := yaml.Unmarshal([]byte(data), ptr); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", fieldName, err))
			}
		}
	})

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	if errs := config.Validate(nil); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	return &config, nil
}

func ToConfigMapData(profile *Profile) (map[string]string, error) {
	if profile == nil {
		return nil, errors.New("cannot marshal nil profile")
	}
	if errs := profile.Validate(nil); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	data := make(map[string]string)

	var errs []error
	forEachConfigMapEntry(profile, func(fieldName string, ptr any) {
		if reflect.ValueOf(ptr).Elem().IsZero() {
			return
		}
		bytes, err := json.Marshal(ptr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", fieldName, err))
			return
		}

		data[fieldName] = string(bytes)
	})

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return data, nil
}

func forEachConfigMapEntry(profile *Profile, f func(fieldName string, ptr any)) {
	for fieldName, ptr := range map[string]any{
		"apiServerAddresses":     &profile.APIServerAddresses,
		"kubeletConfiguration":   &profile.KubeletConfiguration,
		"nodeLocalLoadBalancing": &profile.NodeLocalLoadBalancing,
		"konnectivity":           &profile.Konnectivity,
		"pauseImage":             &profile.PauseImage,
		"dualStackEnabled":       &profile.DualStackEnabled,
	} {
		f(fieldName, ptr)
	}
}
