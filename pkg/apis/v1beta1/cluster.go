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

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/k0sproject/k0s/internal/util"
)

// ClusterConfig cluster manifest
type ClusterConfig struct {
	APIVersion string             `yaml:"apiVersion" validate:"eq=k0s.k0sproject.io/v1beta1"`
	Extensions *ClusterExtensions `yaml:"extensions,omitempty"`
	Images     *ClusterImages     `yaml:"images"`
	Install    *InstallSpec       `yaml:"installConfig,omitempty"`
	Kind       string             `yaml:"kind" validate:"eq=Cluster"`
	Metadata   *ClusterMeta       `yaml:"metadata"`
	Spec       *ClusterSpec       `yaml:"spec"`
	Telemetry  *ClusterTelemetry  `yaml:"telemetry"`
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
}

// APISpec ...
type APISpec struct {
	Address         string            `yaml:"address"`
	ExternalAddress string            `yaml:"externalAddress,omitempty"`
	SANs            []string          `yaml:"sans"`
	ExtraArgs       map[string]string `yaml:"extraArgs,omitempty"`
}

// ControllerManagerSpec ...
type ControllerManagerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// IsZero needed to omit empty object from yaml output
func (c *ControllerManagerSpec) IsZero() bool {
	return len(c.ExtraArgs) == 0
}

// SchedulerSpec ...
type SchedulerSpec struct {
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// IsZero needed to omit empty object from yaml output
func (s *SchedulerSpec) IsZero() bool {
	return len(s.ExtraArgs) == 0
}

// InstallSpec defines the required fields for the `k0s install` command
type InstallSpec struct {
	SystemUsers *SystemUser `yaml:"users,omitempty"`
}

// Validate validates cluster config
func (c *ClusterConfig) Validate() []error {
	var errors []error

	errors = append(errors, c.Spec.Network.Validate()...)
	errors = append(errors, c.Spec.WorkerProfiles.Validate()...)
	// TODO We need to validate all other parts too

	return errors
}

// APIAddress ...
func (a *APISpec) APIAddress() string {
	if a.ExternalAddress != "" {
		return a.ExternalAddress
	}
	return a.Address
}

func (a *APISpec) APIAddressURL() string {
	if a.ExternalAddress != "" {
		return fmt.Sprintf("https://%s:6443", a.ExternalAddress)
	}
	return fmt.Sprintf("https://%s:6443", a.Address)
}

// K0sControlPlaneAPIAddress returns the controller join APIs address
func (a *APISpec) K0sControlPlaneAPIAddress() string {
	if a.ExternalAddress != "" {
		return fmt.Sprintf("https://%s:9443", a.ExternalAddress)
	}
	return fmt.Sprintf("https://%s:9443", a.Address)
}

// Sans return the given SANS plus all local adresses and externalAddress if given
func (a *APISpec) Sans() []string {
	sans, _ := util.AllAddresses()
	sans = append(sans, a.Address)
	sans = append(sans, a.SANs...)
	if a.ExternalAddress != "" {
		sans = append(sans, a.ExternalAddress)
	}

	return util.Unique(sans)
}

// FromYamlFile ...
func FromYamlFile(filename string) (*ClusterConfig, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config file at %s", filename)
	}

	return FromYamlString(string(buf))
}

// FromYamlPipe
func FromYamlPipe(r io.Reader) (*ClusterConfig, error) {
	input, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil
	}
	return FromYamlString(string(input))

}

// FromYamlString
func FromYamlString(yml string) (*ClusterConfig, error) {
	config := &ClusterConfig{}
	err := yaml.Unmarshal([]byte(yml), &config)
	if err != nil {
		return config, err
	}

	if config.Spec == nil {
		config.Spec = DefaultClusterSpec()
	}

	return config, nil
}

// DefaultClusterConfig ...
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		APIVersion: "k0s.k0sproject.io/v1beta1",
		Kind:       "Cluster",
		Metadata: &ClusterMeta{
			Name: "k0s",
		},
		Install:   DefaultInstallSpec(),
		Spec:      DefaultClusterSpec(),
		Images:    DefaultClusterImages(),
		Telemetry: DefaultClusterTelemetry(),
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.Kind = "Cluster"
	c.Metadata = &ClusterMeta{
		Name: "k0s",
	}
	c.Spec = DefaultClusterSpec()
	c.Images = DefaultClusterImages()
	c.Telemetry = DefaultClusterTelemetry()

	type yclusterconfig ClusterConfig
	yc := (*yclusterconfig)(c)

	if err := unmarshal(yc); err != nil {
		return err
	}

	return nil
}

// DefaultAPISpec default settings
func DefaultAPISpec() *APISpec {
	// Collect all nodes addresses for sans
	addresses, _ := util.AllAddresses()
	publicAddress, _ := util.FirstPublicAddress()
	return &APISpec{
		SANs:      addresses,
		Address:   publicAddress,
		ExtraArgs: make(map[string]string),
	}
}

// DefaultClusterSpec default settings
func DefaultClusterSpec() *ClusterSpec {
	return &ClusterSpec{
		Storage: DefaultStorageSpec(),
		Network: DefaultNetwork(),
		API:     DefaultAPISpec(),
		ControllerManager: &ControllerManagerSpec{
			ExtraArgs: make(map[string]string),
		},
		Scheduler: &SchedulerSpec{
			ExtraArgs: make(map[string]string),
		},
		PodSecurityPolicy: DefaultPodSecurityPolicy(),
	}
}
