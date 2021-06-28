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
	"github.com/k0sproject/k0s/internal/util"
	"io/ioutil"
	"os"

	"github.com/k0sproject/k0s/pkg/constant"
)

// ClusterConfig cluster manifest
type ClusterConfig struct {
	APIVersion string       `yaml:"apiVersion" validate:"eq=k0s.k0sproject.io/v1beta1"`
	Kind       string       `yaml:"kind" validate:"eq=Cluster"`
	Metadata   *ClusterMeta `yaml:"metadata"`
	Spec       *ClusterSpec `yaml:"spec"`
	k0sVars    constant.CfgVars
}

// ClusterMeta ...
type ClusterMeta struct {
	Name string `yaml:"name" validate:"required"`
}

// ClusterSpec ...
type ClusterSpec struct {
	API               *APISpec               `yaml:"api"`
	ControllerManager *ControllerManagerSpec `yaml:"controllerManager,omitempty"`
	Scheduler         *SchedulerSpec         `yaml:"scheduler,omitempty"`
	Storage           *StorageSpec           `yaml:"storage"`
	Network           *Network               `yaml:"network"`
	PodSecurityPolicy *PodSecurityPolicy     `yaml:"podSecurityPolicy"`
	WorkerProfiles    WorkerProfiles         `yaml:"workerProfiles,omitempty"`
	Telemetry         *ClusterTelemetry      `yaml:"telemetry"`
	Install           *InstallSpec           `yaml:"installConfig,omitempty"`
	Images            *ClusterImages         `yaml:"images"`
	Extensions        *ClusterExtensions     `yaml:"extensions,omitempty"`
	Konnectivity      *KonnectivitySpec      `yaml:"konnectivity,omitempty"`
}

var _ Validateable = (*ControllerManagerSpec)(nil)

// ControllerManagerSpec ...
type ControllerManagerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// IsZero needed to omit empty object from yaml output
func (c *ControllerManagerSpec) IsZero() bool {
	return len(c.ExtraArgs) == 0
}

func (c *ControllerManagerSpec) Validate() []error {
	return nil
}

var _ Validateable = (*SchedulerSpec)(nil)

// SchedulerSpec ...
type SchedulerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// IsZero needed to omit empty object from yaml output
func (s *SchedulerSpec) IsZero() bool {
	return len(s.ExtraArgs) == 0
}

func (s *SchedulerSpec) Validate() []error {
	return nil
}

var _ Validateable = (*InstallSpec)(nil)

// InstallSpec defines the required fields for the `k0s install` command
type InstallSpec struct {
	SystemUsers *SystemUser `yaml:"users,omitempty"`
}

// Validate stub for Validateable interface
func (i *InstallSpec) Validate() []error {
	return nil
}

// Validateable interface to ensure that all config components implement Validate function
type Validateable interface {
	Validate() []error
}

// Validate validates cluster config
func (c *ClusterConfig) Validate() []error {
	var errors []error

	errors = append(errors, validateSpecs(c.Spec.API)...)
	errors = append(errors, validateSpecs(c.Spec.ControllerManager)...)
	errors = append(errors, validateSpecs(c.Spec.Scheduler)...)
	errors = append(errors, validateSpecs(c.Spec.Storage)...)
	errors = append(errors, validateSpecs(c.Spec.Network)...)
	errors = append(errors, validateSpecs(c.Spec.PodSecurityPolicy)...)
	errors = append(errors, validateSpecs(c.Spec.WorkerProfiles)...)
	errors = append(errors, validateSpecs(c.Spec.Telemetry)...)
	errors = append(errors, validateSpecs(c.Spec.Install)...)
	errors = append(errors, validateSpecs(c.Spec.Extensions)...)
	errors = append(errors, validateSpecs(c.Spec.Konnectivity)...)

	return errors
}

// validateSpecs invokes validator Validate function
func validateSpecs(v Validateable) []error {
	return v.Validate()
}

// ConfigFromFile ...
func ConfigFromFile(filename string, k0sVars constant.CfgVars) (*ClusterConfig, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at %s: %w", filename, err)
	}

	return configFromString(string(buf), k0sVars)
}

// ConfigFromStdin tries to read k0s.yaml config from stdin
func ConfigFromStdin(k0sVars constant.CfgVars) (*ClusterConfig, error) {
	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("can't read configration from stdin: %v", err)
	}
	return configFromString(string(input), k0sVars)

}

func configFromString(yml string, k0sVars constant.CfgVars) (*ClusterConfig, error) {
	config := &ClusterConfig{k0sVars: k0sVars}
	err := util.YamlUnmarshalStrictIgnoringFields([]byte(yml), &config, []string{"interval"})
	if err != nil {
		return config, err
	}

	if config.Spec == nil {
		config.Spec = DefaultClusterSpec(k0sVars)
	}

	return config, nil
}

// DefaultClusterConfig ...
func DefaultClusterConfig(k0sVars constant.CfgVars) *ClusterConfig {
	return &ClusterConfig{
		APIVersion: "k0s.k0sproject.io/v1beta1",
		Kind:       "Cluster",
		Metadata: &ClusterMeta{
			Name: "k0s",
		},
		Spec: DefaultClusterSpec(k0sVars),
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Kind = "Cluster"
	c.Metadata = &ClusterMeta{
		Name: "k0s",
	}
	c.Spec = DefaultClusterSpec(c.k0sVars)

	type yclusterconfig ClusterConfig
	yc := (*yclusterconfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

// DefaultClusterSpec default settings
func DefaultClusterSpec(k0sVars constant.CfgVars) *ClusterSpec {
	return &ClusterSpec{
		Storage: DefaultStorageSpec(k0sVars),
		Network: DefaultNetwork(),
		API:     DefaultAPISpec(),
		ControllerManager: &ControllerManagerSpec{
			ExtraArgs: make(map[string]string),
		},
		Scheduler: &SchedulerSpec{
			ExtraArgs: make(map[string]string),
		},
		PodSecurityPolicy: DefaultPodSecurityPolicy(),
		Install:           DefaultInstallSpec(),
		Images:            DefaultClusterImages(),
		Telemetry:         DefaultClusterTelemetry(),
		Konnectivity:      DefaultKonnectivitySpec(),
	}
}
