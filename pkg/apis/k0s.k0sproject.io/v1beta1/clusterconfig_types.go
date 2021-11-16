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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/strictyaml"
	"github.com/k0sproject/k0s/pkg/constant"
)

const (
	ClusterConfigKind       = "ClusterConfig"
	ClusterConfigAPIVersion = "k0s.k0sproject.io/v1beta1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterSpec defines the desired state of ClusterConfig
type ClusterSpec struct {
	API               *APISpec               `json:"api,omitempty"`
	ControllerManager *ControllerManagerSpec `json:"controllerManager,omitempty"`
	Scheduler         *SchedulerSpec         `json:"scheduler,omitempty"`
	Storage           *StorageSpec           `json:"storage,omitempty"`
	Network           *Network               `json:"network,omitempty"`
	PodSecurityPolicy *PodSecurityPolicy     `json:"podSecurityPolicy,omitempty"`
	WorkerProfiles    WorkerProfiles         `json:"workerProfiles,omitempty"`
	Telemetry         *ClusterTelemetry      `json:"telemetry,omitempty"`
	Install           *InstallSpec           `json:"installConfig,omitempty"`
	Images            *ClusterImages         `json:"images,omitempty"`
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
//+kubebuilder:validation:Optional
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
// +groupName=k0s.k0sproject.io

// ClusterConfig is the Schema for the clusterconfigs API
type ClusterConfig struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`

	DataDir string              `json:"dataDir,omitempty"`
	Spec    *ClusterSpec        `json:"spec,omitempty"`
	Status  ClusterConfigStatus `json:"status,omitempty"`
}

// StripDefaults returns a copy of the config where the default values a nilled out
func (c *ClusterConfig) StripDefaults() *ClusterConfig {
	copy := c.DeepCopy()
	if reflect.DeepEqual(copy.Spec.API, DefaultAPISpec()) {
		copy.Spec.API = nil
	}
	if reflect.DeepEqual(copy.Spec.ControllerManager, DefaultControllerManagerSpec()) {
		copy.Spec.ControllerManager = nil
	}
	if reflect.DeepEqual(copy.Spec.Scheduler, DefaultSchedulerSpec()) {
		copy.Spec.Scheduler = nil
	}
	if reflect.DeepEqual(c.Spec.Storage, DefaultStorageSpec(constant.DataDirDefault)) {
		c.Spec.ControllerManager = nil
	}
	if reflect.DeepEqual(copy.Spec.Network, DefaultNetwork()) {
		copy.Spec.Network = nil
	}
	if reflect.DeepEqual(copy.Spec.PodSecurityPolicy, DefaultPodSecurityPolicy()) {
		copy.Spec.PodSecurityPolicy = nil
	}
	if reflect.DeepEqual(copy.Spec.Telemetry, DefaultClusterTelemetry()) {
		copy.Spec.Telemetry = nil
	}
	if reflect.DeepEqual(copy.Spec.Images, DefaultClusterImages()) {
		copy.Spec.Images = nil
	}
	if reflect.DeepEqual(copy.Spec.Konnectivity, DefaultKonnectivitySpec()) {
		copy.Spec.Konnectivity = nil
	}
	return copy
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

func DefaultControllerManagerSpec() *ControllerManagerSpec {
	return &ControllerManagerSpec{
		ExtraArgs: make(map[string]string),
	}
}

// SchedulerSpec defines the fields for the Scheduler
type SchedulerSpec struct {
	// Map of key-values (strings) for any extra arguments you want to pass down to Kubernetes scheduler process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

func DefaultSchedulerSpec() *SchedulerSpec {
	return &SchedulerSpec{
		ExtraArgs: make(map[string]string),
	}
}

//+kubebuilder:object:root=true
// +genclient
// +genclient:onlyVerbs=create
// ClusterConfigList contains a list of ClusterConfig
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{}, &ClusterConfigList{})
}

var _ Validateable = (*ControllerManagerSpec)(nil)

// IsZero needed to omit empty object from yaml output
func (c *ControllerManagerSpec) IsZero() bool {
	return len(c.ExtraArgs) == 0
}

// IsZero needed to omit empty object from yaml output
func (s *SchedulerSpec) IsZero() bool {
	return len(s.ExtraArgs) == 0
}

// ConfigFromFile takes a file path as Input, and parses it into a ClusterConfig
func ConfigFromFile(filename string, dataDir string) (*ClusterConfig, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file at %s: %w", filename, err)
	}
	return ConfigFromString(string(buf), dataDir)
}

// ConfigFromStdin tries to read k0s.yaml config from stdin
func ConfigFromStdin(dataDir string) (*ClusterConfig, error) {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("can't read configration from stdin: %v", err)
	}
	return ConfigFromString(string(input), dataDir)
}

func ConfigFromString(yml string, dataDir string) (*ClusterConfig, error) {
	config := DefaultClusterConfig(dataDir)
	err := strictyaml.YamlUnmarshalStrictIgnoringFields([]byte(yml), config, "interval")
	if err != nil {
		return config, err
	}
	if config.Spec == nil {
		config.Spec = DefaultClusterSpec(dataDir)
	}
	return config, nil
}

// DefaultClusterConfig sets the default ClusterConfig values, when none are given
func DefaultClusterConfig(dataDir string) *ClusterConfig {
	return &ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "k0s"},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k0s.k0sproject.io/v1beta1",
			Kind:       "ClusterConfig",
		},
		Spec: DefaultClusterSpec(dataDir),
	}
}

// UnmarshalJSON sets in some sane defaults when unmarshaling the data from json
func (c *ClusterConfig) UnmarshalJSON(data []byte) error {
	if c.Kind == "" {
		c.Kind = "ClusterConfig"
	}
	if c.ClusterName == "" {
		c.ClusterName = "k0s"
	}
	c.Spec = DefaultClusterSpec(c.DataDir)

	type config ClusterConfig
	jc := (*config)(c)

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	return decoder.Decode(jc)
}

// GetBootstrappingConfig returns a ClusterConfig object stripped of Cluster-Wide Settings
func (c *ClusterConfig) GetBootstrappingConfig() *ClusterConfig {
	return &ClusterConfig{
		ObjectMeta: c.ObjectMeta,
		TypeMeta:   c.TypeMeta,
		DataDir:    c.DataDir,
		Spec: &ClusterSpec{
			API: &APISpec{
				Address:    c.Spec.API.Address,
				ExtraArgs:  c.Spec.API.ExtraArgs,
				K0sAPIPort: c.Spec.API.K0sAPIPort,
				Port:       c.Spec.API.Port,
				SANs:       c.Spec.API.SANs,
			},
			Storage: &StorageSpec{
				Type: c.Spec.Storage.Type,
				Etcd: &EtcdConfig{
					PeerAddress: c.Spec.Storage.Etcd.PeerAddress,
				},
				Kine: c.Spec.Storage.Kine,
			},
			Network: &Network{
				ServiceCIDR: c.Spec.Network.ServiceCIDR,
			},
			Install: c.Spec.Install,
		},
		Status: c.Status,
	}
}

// HACK: the current ClusterConfig struct holds both bootstrapping config & cluster-wide config
// this hack strips away the node-specific bootstrapping config so that we write a "clean" config to the CR
// This function accepts a standard ClusterConfig and returns the same config minus the node specific info:
// - APISpec
// - StorageSpec
// - Network.ServiceCIDR
// - Install
func (c *ClusterConfig) GetClusterWideConfig() *ClusterConfig {
	return &ClusterConfig{
		ObjectMeta: c.ObjectMeta,
		TypeMeta:   c.TypeMeta,
		DataDir:    c.DataDir,
		Spec: &ClusterSpec{
			API: &APISpec{
				ExternalAddress: c.Spec.API.ExternalAddress,
			},
			ControllerManager: c.Spec.ControllerManager,
			Scheduler:         c.Spec.Scheduler,
			Network: &Network{
				Calico:     c.Spec.Network.Calico,
				DualStack:  c.Spec.Network.DualStack,
				KubeProxy:  c.Spec.Network.KubeProxy,
				KubeRouter: c.Spec.Network.KubeRouter,
				PodCIDR:    c.Spec.Network.PodCIDR,
				Provider:   c.Spec.Network.Provider,
			},
			PodSecurityPolicy: c.Spec.PodSecurityPolicy,
			WorkerProfiles:    c.Spec.WorkerProfiles,
			Telemetry:         c.Spec.Telemetry,
			Images:            c.Spec.Images,
			Extensions:        c.Spec.Extensions,
			Konnectivity:      c.Spec.Konnectivity,
		},
		Status: c.Status,
	}
}

// DefaultClusterSpec default settings
func DefaultClusterSpec(dataDir string) *ClusterSpec {
	return &ClusterSpec{
		Storage:           DefaultStorageSpec(dataDir),
		Network:           DefaultNetwork(),
		API:               DefaultAPISpec(),
		ControllerManager: DefaultControllerManagerSpec(),
		Scheduler:         DefaultSchedulerSpec(),
		PodSecurityPolicy: DefaultPodSecurityPolicy(),
		Install:           DefaultInstallSpec(),
		Images:            DefaultClusterImages(),
		Telemetry:         DefaultClusterTelemetry(),
		Konnectivity:      DefaultKonnectivitySpec(),
	}
}

func (c *ControllerManagerSpec) Validate() []error {
	return nil
}

var _ Validateable = (*SchedulerSpec)(nil)

func (s *SchedulerSpec) Validate() []error {
	return nil
}

var _ Validateable = (*InstallSpec)(nil)

// Validate stub for Validateable interface
func (i *InstallSpec) Validate() []error {
	return nil
}

// Validateable interface to ensure that all config components implement Validate function
// +k8s:deepcopy-gen=false
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

// CRValidator is used to make sure a config CR is created with correct values
func (c *ClusterConfig) CRValidator() *ClusterConfig {
	copy := c.DeepCopy()
	copy.ClusterName = "k0s"
	copy.ObjectMeta.Name = "k0s"
	copy.ObjectMeta.Namespace = "kube-system"

	return copy
}

// validateSpecs invokes validator Validate function
func validateSpecs(v Validateable) []error {
	return v.Validate()
}
