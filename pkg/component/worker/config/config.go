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
	"fmt"
	"reflect"

	kubeletv1beta1 "k8s.io/kubelet/config/v1beta1"

	"go.uber.org/multierr"
	"sigs.k8s.io/yaml"
)

type Profile struct {
	KubeletConfiguration kubeletv1beta1.KubeletConfiguration
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
	p.KubeletConfiguration.DeepCopyInto(&out.KubeletConfiguration)
}

func FromConfigMapData(data map[string]string) (*Profile, error) {
	var config Profile
	var errs error
	forEachConfigMapEntry(&config, func(fieldName string, ptr any) {
		data, ok := data[fieldName]
		if ok {
			if err := yaml.Unmarshal([]byte(data), ptr); err != nil {
				errs = multierr.Append(errs, fmt.Errorf("%s: %w", fieldName, err))
			}
		}
	})

	if errs != nil {
		return nil, errs
	}

	return &config, nil
}

func ToConfigMapData(profile *Profile) (map[string]string, error) {
	data := make(map[string]string)

	if profile == nil {
		return data, nil
	}

	var errs error
	forEachConfigMapEntry(profile, func(fieldName string, ptr any) {
		if reflect.ValueOf(ptr).Elem().IsZero() {
			return
		}
		bytes, err := json.Marshal(ptr)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("%s: %w", fieldName, err))
			return
		}

		data[fieldName] = string(bytes)
	})

	if errs != nil {
		return nil, errs
	}

	return data, nil
}

func forEachConfigMapEntry(profile *Profile, f func(fieldName string, ptr any)) {
	for fieldName, ptr := range map[string]any{
		"kubeletConfiguration": &profile.KubeletConfiguration,
	} {
		f(fieldName, ptr)
	}
}
