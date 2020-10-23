/*
Copyright 2020 Mirantis, Inc.

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

import "github.com/Mirantis/mke/pkg/constant"

// PodSecurityPolicy defines the config options for setting system level default PSP
type PodSecurityPolicy struct {
	DefaultPolicy string `yaml:"defaultPolicy"`
}

// DefaultPodSecurityPolicy creates new PodSecurityPolicy with sane defaults
func DefaultPodSecurityPolicy() *PodSecurityPolicy {
	return &PodSecurityPolicy{
		DefaultPolicy: constant.DefaultPSP,
	}
}
