// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k0sproject/k0s/internal/pkg/strictyaml"
	"github.com/k0sproject/k0s/pkg/constant"
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
	API               *APISpec               `json:"api,omitempty"`
	ControllerManager *ControllerManagerSpec `json:"controllerManager,omitempty"`
	Scheduler         *SchedulerSpec         `json:"scheduler,omitempty"`
	Storage           *StorageSpec           `json:"storage,omitempty"`
	Network           *Network               `json:"network,omitempty"`
	WorkerProfiles    WorkerProfiles         `json:"workerProfiles,omitempty"`
	Telemetry         *ClusterTelemetry      `json:"telemetry,omitempty"`
	Install           *InstallSpec           `json:"installConfig,omitempty"`
	Images            *ClusterImages         `json:"images,omitempty"`
	Extensions        *ClusterExtensions     `json:"extensions,omitempty"`
	Konnectivity      *KonnectivitySpec      `json:"konnectivity,omitempty"`
	FeatureGates      FeatureGates           `json:"featureGates,omitempty"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig
type ClusterConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// ClusterConfig is the Schema for the clusterconfigs API
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
type ClusterConfig struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	Spec   *ClusterSpec         `json:"spec,omitempty"`
	Status *ClusterConfigStatus `json:"status,omitempty"`
}

// StripDefaults returns a copy of the config where the default values a nilled out
func (c *ClusterConfig) StripDefaults() *ClusterConfig {
	c = c.DeepCopy() // Clone and overwrite receiver to avoid side effects
	if c == nil || c.Spec == nil {
		return c
	}
	if reflect.DeepEqual(c.Spec.API, DefaultAPISpec()) {
		c.Spec.API = nil
	}
	if reflect.DeepEqual(c.Spec.ControllerManager, DefaultControllerManagerSpec()) {
		c.Spec.ControllerManager = nil
	}
	if reflect.DeepEqual(c.Spec.Scheduler, DefaultSchedulerSpec()) {
		c.Spec.Scheduler = nil
	}
	if reflect.DeepEqual(c.Spec.Storage, DefaultStorageSpec()) {
		c.Spec.Storage = nil
	}
	if reflect.DeepEqual(c.Spec.Network, DefaultNetwork()) {
		c.Spec.Network = nil
	} else if c.Spec.Network != nil &&
		c.Spec.Network.NodeLocalLoadBalancing != nil &&
		c.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy != nil &&
		reflect.DeepEqual(c.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image, DefaultEnvoyProxyImage()) {
		c.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image = nil
	}
	if reflect.DeepEqual(c.Spec.Telemetry, DefaultClusterTelemetry()) {
		c.Spec.Telemetry = nil
	}
	if reflect.DeepEqual(c.Spec.Images, DefaultClusterImages()) {
		c.Spec.Images = nil
	} else {
		stripDefaultImages(c.Spec.Images, DefaultClusterImages())
	}
	if reflect.DeepEqual(c.Spec.Konnectivity, DefaultKonnectivitySpec()) {
		c.Spec.Konnectivity = nil
	}
	return c
}

func stripDefaultImages(cfgImages, defaultImages *ClusterImages) {
	if cfgImages != nil && defaultImages != nil {
		cfgVal := reflect.ValueOf(cfgImages).Elem()
		defaultVal := reflect.ValueOf(defaultImages).Elem()
		stripDefaults(cfgVal, defaultVal)
	}
}

// Zeroes out any field in actualValue whose value equals the corresponding
// field in defaultValue, but only if that field's JSON tag contains
// "omitempty". Both actualValue and defaultValue must be wrapping the same
// struct type, actualValue must be addressable so its fields can be set, and
// defaultValue is never modified. Unexported fields and fields without
// "omitempty" (including json:\"-\") are left untouched.
//
// This logic will be applied recursively, i.e. stripDefaults will be called on
// nested structs (or pointers to them). All other types will be handled at the
// top level only.
func stripDefaults(actualValue, defaultValue reflect.Value) {
	typ := actualValue.Type()
	for i := range typ.NumField() {
		field := typ.Field(i)

		switch field.Type.Kind() {
		case reflect.Pointer:
			// Skip fields to be ignored.
			if !field.IsExported() || !canStrip(field) {
				continue
			}

			actualValue, defaultValue := actualValue.Field(i), defaultValue.Field(i)

			// Skip over nil pointers.
			if actualValue.IsNil() || defaultValue.IsNil() {
				continue
			}

			// Dereference pointers.
			actualElem, defaultElem := actualValue.Elem(), defaultValue.Elem()

			if reflect.DeepEqual(actualElem.Interface(), defaultElem.Interface()) {
				// Underlying values are equal, nil out pointer.
				actualValue.SetZero()
			} else if actualElem.Kind() == reflect.Struct {
				// Underlying values are different, recurse into the pointed struct.
				stripDefaults(actualElem, defaultElem)
				// Nil out pointer if only the zero value remains.
				if actualElem.IsZero() {
					actualValue.SetZero()
				}
			}

		case reflect.Struct:
			// Recurse into structs. The omitempty tag is meaningless for them.
			if field.IsExported() {
				stripDefaults(actualValue.Field(i), defaultValue.Field(i))
			}

		default:
			// Skip fields to be ignored.
			if !field.IsExported() || !canStrip(field) {
				continue
			}

			actualValue, defaultValue := actualValue.Field(i), defaultValue.Field(i)
			if reflect.DeepEqual(actualValue.Interface(), defaultValue.Interface()) {
				actualValue.SetZero()
			}
		}
	}
}

// Indicates whether a struct field is eligible for stripping defaults: it
// returns true if the JSON tag includes "omitempty" and the field is not
// explicitly ignored. Fields tagged `json:"-"` (or `json:"-,omitempty"`) are
// never stripped.
func canStrip(f reflect.StructField) bool {
	if name, tags, hasTags := strings.Cut(f.Tag.Get("json"), ","); hasTags && name != "-" {
		tags := strings.Split(tags, ",")
		return slices.Contains(tags, "omitempty")
	}

	return false
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

	// Slice of strings with raw arguments to pass to the kube-controller-manager process
	// These arguments will be appended to the `ExtraArgs` and aren't validated at all.
	RawArgs []string `json:"rawArgs,omitempty"`
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

	// Slice of strings with raw arguments to pass to the Kubernetes scheduler process
	// These arguments will be appended to the `ExtraArgs` and aren't validated at all.
	// ExtraArgs are recommended over RawArgs. If possible use ExtraArgs to set arguments.
	RawArgs []string `json:"rawArgs,omitempty"`
}

func DefaultSchedulerSpec() *SchedulerSpec {
	return &SchedulerSpec{
		ExtraArgs: make(map[string]string),
	}
}

var _ Validateable = (*SchedulerSpec)(nil)

func (*SchedulerSpec) Validate() []error { return nil }

// +kubebuilder:object:root=true
// ClusterConfigList contains a list of ClusterConfig
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterConfig `json:"items"`
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

func ConfigFromBytes(bytes []byte) (*ClusterConfig, error) {
	return DefaultClusterConfig().MergedWithYAML(bytes)
}

func (c *ClusterConfig) MergedWithYAML(bytes []byte) (*ClusterConfig, error) {
	merged := c.DeepCopy()
	err := strictyaml.YamlUnmarshalStrictIgnoringFields(bytes, merged, "interval", "podSecurityPolicy")
	if err != nil {
		return nil, err
	}
	if merged.Spec == nil {
		merged.Spec = c.Spec
	}
	return merged, nil
}

// DefaultClusterConfig sets the default ClusterConfig values, when none are given
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "ClusterConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.ClusterConfigObjectName,
			Namespace: constant.ClusterConfigNamespace,
		},
		Spec: DefaultClusterSpec(),
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
	c.Spec = DefaultClusterSpec()
	if storage != nil {
		c.Spec.Storage = storage
	}

	type config ClusterConfig
	jc := (*config)(c)

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	err := decoder.Decode(jc)
	if err != nil {
		return err
	}

	if jc.Spec == nil {
		jc.Spec = DefaultClusterSpec()
		if storage != nil {
			jc.Spec.Storage = storage
		}
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
func DefaultClusterSpec() *ClusterSpec {
	spec := &ClusterSpec{
		Extensions:        DefaultExtensions(),
		Storage:           DefaultStorageSpec(),
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
		"telemetry":         s.Telemetry,
		"install":           s.Install,
		"extensions":        s.Extensions,
		"konnectivity":      s.Konnectivity,
	} {
		for _, err := range field.Validate() {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	errs = append(errs, s.WorkerProfiles.Validate(field.NewPath("workerProfiles"))...)

	for _, err := range s.Images.Validate(field.NewPath("images")) {
		errs = append(errs, err)
	}

	for _, err := range s.ValidateNodeLocalLoadBalancing() {
		errs = append(errs, err)
	}

	if s.Network != nil && s.Network.ControlPlaneLoadBalancing != nil {
		for _, err := range s.Network.ControlPlaneLoadBalancing.Validate() {
			errs = append(errs, fmt.Errorf("controlPlaneLoadBalancing: %w", err))
		}
	}

	return
}

// APIServerURLForHostNetworkPods returns the effective API server URL for
// host-network components running on worker nodes (like kube-proxy and kube-router).
// When node-local load balancing is enabled, it returns the localhost URL;
// otherwise, it returns the standard API server URL.
func (s *ClusterSpec) APIServerURLForHostNetworkPods() string {
	if s.Network != nil {
		nllb := s.Network.NodeLocalLoadBalancing
		if nllb.IsEnabled() && nllb.Type == NllbTypeEnvoyProxy {
			return fmt.Sprintf("https://localhost:%d", nllb.EnvoyProxy.APIServerBindPort)
		}
	}
	return s.API.APIAddressURL()
}

func (s *ClusterSpec) ValidateNodeLocalLoadBalancing() (errs field.ErrorList) {
	if s.Network == nil || !s.Network.NodeLocalLoadBalancing.IsEnabled() {
		return
	}

	if s.API == nil {
		return
	}

	path := field.NewPath("network", "nodeLocalLoadBalancing", "enabled")

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

// HACK: the current ClusterConfig struct holds both bootstrapping config & cluster-wide config
// this hack strips away the node-specific bootstrapping config so that we write a "clean" config to the CR
// This function accepts a standard ClusterConfig and returns the same config minus the node specific info:
// - APISpec
// - StorageSpec
// - Network.ServiceCIDR
// - Network.ClusterDomain
// - Network.ControlPlaneLoadBalancing
// - Network.PrimaryAddressFamily
// - Install
func (c *ClusterConfig) GetClusterWideConfig() *ClusterConfig {
	c = c.DeepCopy()
	if c != nil && c.Spec != nil {
		c.Spec.API = nil
		c.Spec.Storage = nil
		if c.Spec.Network != nil {
			c.Spec.Network.ServiceCIDR = ""
			c.Spec.Network.ClusterDomain = ""
			c.Spec.Network.ControlPlaneLoadBalancing = nil
			c.Spec.Network.PrimaryAddressFamily = ""
		}
		c.Spec.Install = nil
	}

	return c
}

// CRValidator is used to make sure a config CR is created with correct values
func (c *ClusterConfig) CRValidator() *ClusterConfig {
	copy := c.DeepCopy()
	copy.Name = "k0s"
	copy.Namespace = metav1.NamespaceSystem

	return copy
}

func (c *ClusterConfig) PrimaryAddressFamily() PrimaryAddressFamilyType {
	if c != nil && c.Spec != nil && c.Spec.Network != nil && c.Spec.Network.PrimaryAddressFamily != PrimaryFamilyUnknown {
		return c.Spec.Network.PrimaryAddressFamily
	}
	return c.Spec.API.DetectPrimaryAddressFamily()
}
