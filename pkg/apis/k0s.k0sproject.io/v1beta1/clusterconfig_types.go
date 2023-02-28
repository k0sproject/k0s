/*
Copyright 2020 k0s authors

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/strictyaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	ClusterConfigKind       = "ClusterConfig"
	ClusterConfigAPIVersion = "k0s.k0sproject.io/v1beta1"
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
	WorkerProfiles    WorkerProfiles         `json:"workerProfiles,omitempty"`
	Telemetry         *ClusterTelemetry      `json:"telemetry"`
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

	Spec   *ClusterSpec        `json:"spec,omitempty"`
	Status ClusterConfigStatus `json:"status,omitempty"`
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
	if reflect.DeepEqual(c.Spec.Storage, DefaultStorageSpec()) {
		c.Spec.Storage = nil
	}
	if reflect.DeepEqual(copy.Spec.Network, DefaultNetwork()) {
		copy.Spec.Network = nil
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

var _ Validateable = (*InstallSpec)(nil)

func (*InstallSpec) Validate() []error { return nil }

// ControllerManagerSpec defines the fields for the ControllerManager
type ControllerManagerSpec struct {
	// Map of key-values (strings) for any extra arguments you want to pass down to the Kubernetes controller manager process
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

var _ Validateable = (*ControllerManagerSpec)(nil)

func DefaultControllerManagerSpec() *ControllerManagerSpec {
	return &ControllerManagerSpec{
		ExtraArgs: make(map[string]string),
	}
}

func (c *ControllerManagerSpec) Validate() []error { return nil }

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

var _ Validateable = (*SchedulerSpec)(nil)

func (*SchedulerSpec) Validate() []error { return nil }

// +kubebuilder:object:root=true
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

func ConfigFromString(yml string, defaultStorage ...*StorageSpec) (*ClusterConfig, error) {
	config := DefaultClusterConfig(defaultStorage...)
	err := strictyaml.YamlUnmarshalStrictIgnoringFields([]byte(yml), config, "interval", "podSecurityPolicy")
	if err != nil {
		return config, err
	}
	if config.Spec == nil {
		config.Spec = DefaultClusterSpec(defaultStorage...)
	}
	return config, nil
}

// ConfigFromReader reads the configuration from any reader (can be stdin, file reader, etc)
func ConfigFromReader(r io.Reader, defaultStorage ...*StorageSpec) (*ClusterConfig, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ConfigFromString(string(input), defaultStorage...)
}

// DefaultClusterConfig sets the default ClusterConfig values, when none are given
func DefaultClusterConfig(defaultStorage ...*StorageSpec) *ClusterConfig {
	clusterSpec := DefaultClusterSpec(defaultStorage...)
	return &ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "k0s"},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k0s.k0sproject.io/v1beta1",
			Kind:       "ClusterConfig",
		},
		Spec: clusterSpec,
	}
}

// UnmarshalJSON sets in some sane defaults when unmarshaling the data from json
func (c *ClusterConfig) UnmarshalJSON(data []byte) error {
	if c.Kind == "" {
		c.Kind = "ClusterConfig"
	}

	// If there's already a storage configured, do not override it with default
	// etcd config BEFORE unmarshaling
	var storage *StorageSpec
	if c.Spec != nil && c.Spec.Storage != nil {
		storage = c.Spec.Storage
	}
	c.Spec = DefaultClusterSpec(storage)

	type config ClusterConfig
	jc := (*config)(c)

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(jc)
	if err != nil {
		return err
	}

	if jc.Spec == nil {
		jc.Spec = DefaultClusterSpec(storage)
		return nil
	}
	if jc.Spec.Storage == nil {
		jc.Spec.Storage = DefaultStorageSpec()
	}
	if jc.Spec.Extensions == nil {
		jc.Spec.Extensions = DefaultExtensions()
	}
	if jc.Spec.Network == nil {
		jc.Spec.Network = DefaultNetwork()
	}
	if jc.Spec.API == nil {
		jc.Spec.API = DefaultAPISpec()
	}
	if jc.Spec.ControllerManager == nil {
		jc.Spec.ControllerManager = DefaultControllerManagerSpec()
	}
	if jc.Spec.Scheduler == nil {
		jc.Spec.Scheduler = DefaultSchedulerSpec()
	}
	if jc.Spec.Install == nil {
		jc.Spec.Install = DefaultInstallSpec()
	}
	if jc.Spec.Images == nil {
		jc.Spec.Images = DefaultClusterImages()
	}
	if jc.Spec.Telemetry == nil {
		jc.Spec.Telemetry = DefaultClusterTelemetry()
	}
	if jc.Spec.Konnectivity == nil {
		jc.Spec.Konnectivity = DefaultKonnectivitySpec()
	}

	jc.Spec.overrideImageRepositories()

	return nil
}

// DefaultClusterSpec default settings
func DefaultClusterSpec(defaultStorage ...*StorageSpec) *ClusterSpec {
	var storage *StorageSpec
	if defaultStorage == nil || defaultStorage[0] == nil {
		storage = DefaultStorageSpec()
	} else {
		storage = defaultStorage[0]
	}

	spec := &ClusterSpec{
		Extensions:        DefaultExtensions(),
		Storage:           storage,
		Network:           DefaultNetwork(),
		API:               DefaultAPISpec(),
		ControllerManager: DefaultControllerManagerSpec(),
		Scheduler:         DefaultSchedulerSpec(),
		Install:           DefaultInstallSpec(),
		Images:            DefaultClusterImages(),
		Telemetry:         DefaultClusterTelemetry(),
		Konnectivity:      DefaultKonnectivitySpec(),
	}

	spec.overrideImageRepositories()

	return spec
}

// Validateable interface to ensure that all config components implement Validate function
// +k8s:deepcopy-gen=false
type Validateable interface {
	Validate() []error
}

func (s *ClusterSpec) Validate() (errs []error) {
	if s == nil {
		return
	}

	for name, field := range map[string]Validateable{
		"api":               s.API,
		"controllerManager": s.ControllerManager,
		"scheduler":         s.Scheduler,
		"storage":           s.Storage,
		"network":           s.Network,
		"workerProfiles":    s.WorkerProfiles,
		"telemetry":         s.Telemetry,
		"install":           s.Install,
		"extensions":        s.Extensions,
		"konnectivity":      s.Konnectivity,
	} {
		for _, err := range field.Validate() {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	for _, err := range s.Images.Validate(field.NewPath("images")) {
		errs = append(errs, err)
	}

	for _, err := range s.ValidateNodeLocalLoadBalancing() {
		errs = append(errs, err)
	}

	return
}

func (s *ClusterSpec) ValidateNodeLocalLoadBalancing() (errs field.ErrorList) {
	if s.Network == nil || !s.Network.NodeLocalLoadBalancing.IsEnabled() {
		return
	}

	if s.API == nil {
		return
	}

	path := field.NewPath("network", "nodeLocalLoadBalancing", "enabled")
	if s.API.TunneledNetworkingMode {
		detail := "node-local load balancing cannot be used in tunneled networking mode"
		errs = append(errs, field.Forbidden(path, detail))
	}

	if s.API.ExternalAddress != "" {
		detail := "node-local load balancing cannot be used in conjunction with an external Kubernetes API server address"
		errs = append(errs, field.Forbidden(path, detail))
	}

	return
}

func (s *ClusterSpec) overrideImageRepositories() {
	if s != nil &&
		s.Images != nil &&
		s.Images.Repository != "" &&
		s.Network != nil &&
		s.Network.NodeLocalLoadBalancing != nil &&
		s.Network.NodeLocalLoadBalancing.EnvoyProxy != nil &&
		s.Network.NodeLocalLoadBalancing.EnvoyProxy.Image != nil {
		i := s.Network.NodeLocalLoadBalancing.EnvoyProxy.Image
		i.Image = overrideRepository(s.Images.Repository, i.Image)
	}
}

// Validate validates cluster config
func (c *ClusterConfig) Validate() (errs []error) {
	if c == nil {
		return nil
	}

	for _, err := range c.Spec.Validate() {
		errs = append(errs, fmt.Errorf("spec: %w", err))
	}

	return errs
}

// GetBootstrappingConfig returns a ClusterConfig object stripped of Cluster-Wide Settings
func (c *ClusterConfig) GetBootstrappingConfig(storageSpec *StorageSpec) *ClusterConfig {
	var etcdConfig *EtcdConfig
	if storageSpec.Type == EtcdStorageType {
		etcdConfig = &EtcdConfig{
			ExternalCluster: storageSpec.Etcd.ExternalCluster,
			PeerAddress:     storageSpec.Etcd.PeerAddress,
			ExtraArgs:       storageSpec.Etcd.ExtraArgs,
		}
		c.Spec.Storage.Etcd = etcdConfig
	}
	return &ClusterConfig{
		ObjectMeta: c.ObjectMeta,
		TypeMeta:   c.TypeMeta,
		Spec: &ClusterSpec{
			API:     c.Spec.API,
			Storage: storageSpec,
			Network: &Network{
				ServiceCIDR:   c.Spec.Network.ServiceCIDR,
				DualStack:     c.Spec.Network.DualStack,
				ClusterDomain: c.Spec.Network.ClusterDomain,
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
// - Network.ClusterDomain
// - Install
func (c *ClusterConfig) GetClusterWideConfig() *ClusterConfig {
	c = c.DeepCopy()
	if c != nil && c.Spec != nil {
		c.Spec.API = nil
		c.Spec.Storage = nil
		if c.Spec.Network != nil {
			c.Spec.Network.ServiceCIDR = ""
			c.Spec.Network.ClusterDomain = ""
		}
		c.Spec.Install = nil
	}

	return c
}

// CRValidator is used to make sure a config CR is created with correct values
func (c *ClusterConfig) CRValidator() *ClusterConfig {
	copy := c.DeepCopy()
	copy.ObjectMeta.Name = "k0s"
	copy.ObjectMeta.Namespace = "kube-system"

	return copy
}
