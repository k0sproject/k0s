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
	"io/ioutil"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/constant"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of ClusterConfig
type ClusterSpec struct {
	API               *APISpec               `json:"api"`
	ControllerManager *ControllerManagerSpec `json:"controllerManager,omitempty"`
	Scheduler         *SchedulerSpec         `json:"scheduler,omitempty"`
	Storage           *StorageSpec           `json:"storage"`
	Network           *Network               `json:"network"`
	PodSecurityPolicy *PodSecurityPolicy     `json:"podSecurityPolicy"`
	WorkerProfiles    WorkerProfiles         `json:"workerProfiles,omitempty"`
	Telemetry         *ClusterTelemetry      `json:"telemetry"`
	Install           *InstallSpec           `json:"installConfig,omitempty"`
	Images            *ClusterImages         `json:"images"`
	Extensions        *ClusterExtensions     `json:"extensions,omitempty"`
	Konnectivity      *KonnectivitySpec      `json:"konnectivity,omitempty"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig
type ClusterConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterConfig is the Schema for the clusterconfigs API
type ClusterConfig struct {
	APIVersion string `json:"apiVersion" validate:"eq=k0s.k0sproject.io/v1beta1"`

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec    *ClusterSpec        `json:"spec,omitempty"`
	Status  ClusterConfigStatus `json:"status,omitempty"`
	k0sVars constant.CfgVars
}

// InstallSpec defines the required fields for the `k0s install` command
type InstallSpec struct {
	SystemUsers *SystemUser `json:"users,omitempty"`
}

// ControllerManagerSpec defines the fields for the ControllerManager
type ControllerManagerSpec struct {
	// Map of key-values (strings) for any extra arguments you want to pass down to the Kubernetes controller manager process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// SchedulerSpec defines the fields for the Scheduler
type SchedulerSpec struct {
	// Map of key-values (strings) for any extra arguments you want to pass down to Kubernetes scheduler process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterConfigList contains a list of ClusterConfig
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{}, &ClusterConfigList{})
}

// var _ Validateable = (*ControllerManagerSpec)(nil)

// IsZero needed to omit empty object from yaml output
func (c *ControllerManagerSpec) IsZero() bool {
	return len(c.ExtraArgs) == 0
}

// IsZero needed to omit empty object from yaml output
func (s *SchedulerSpec) IsZero() bool {
	return len(s.ExtraArgs) == 0
}

// ConfigFromFile takes a file path as Input, and parses it into a ClusterConfig
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

// DefaultClusterConfig sets the default ClusterConfig values, when none are given
func DefaultClusterConfig(k0sVars constant.CfgVars) *ClusterConfig {
	return &ClusterConfig{
		APIVersion: "k0s.k0sproject.io/v1beta1",
		ObjectMeta: metav1.ObjectMeta{ClusterName: "k0s"},
		Spec:       DefaultClusterSpec(k0sVars),
	}
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (c *ClusterConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	c.ClusterName = "k0s"
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

/*
func (c *ControllerManagerSpec) Validate() []error {
	return nil
}

// var _ Validateable = (*SchedulerSpec)(nil)




func (s *SchedulerSpec) Validate() []error {
	return nil
}


// var _ Validateable = (*InstallSpec)(nil)

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


*/
