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

package v1beta1

import (
	"encoding/json"

	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

// NodeLocalLoadBalancing defines the configuration options related to k0s's
// node-local load balancing feature.
// NOTE: This feature is currently unsupported on ARMv7!
type NodeLocalLoadBalancing struct {
	// enabled indicates if node-local load balancing should be used to access
	// Kubernetes API servers from worker nodes.
	// Default: false
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled"`

	// type indicates the type of the node-local load balancer to deploy on
	// worker nodes. Currently, the only supported type is "EnvoyProxy".
	// +kubebuilder:default=EnvoyProxy
	Type NllbType `json:"type,omitempty"`

	// envoyProxy contains configuration options related to the "EnvoyProxy" type
	// of load balancing.
	EnvoyProxy *EnvoyProxy `json:"envoyProxy,omitempty"`
}

// NllbType describes which type of load balancer should be deployed for the
// node-local load balancing. The default is [NllbTypeEnvoyProxy].
// +kubebuilder:validation:Enum=EnvoyProxy
type NllbType string

const (
	// NllbTypeEnvoyProxy selects Envoy as the backing load balancer.
	NllbTypeEnvoyProxy NllbType = "EnvoyProxy"
)

// DefaultNodeLocalLoadBalancing returns the default node-local load balancing configuration.
func DefaultNodeLocalLoadBalancing() *NodeLocalLoadBalancing {
	var nllb NodeLocalLoadBalancing
	nllb.setDefaults()
	return &nllb
}

var _ json.Unmarshaler = (*NodeLocalLoadBalancing)(nil)

func (n *NodeLocalLoadBalancing) UnmarshalJSON(data []byte) error {
	type nodeLocalLoadBalancing NodeLocalLoadBalancing
	if err := json.Unmarshal(data, (*nodeLocalLoadBalancing)(n)); err != nil {
		return err
	}

	n.setDefaults()

	return nil
}

func (n *NodeLocalLoadBalancing) setDefaults() {
	if n.Type == "" {
		n.Type = NllbTypeEnvoyProxy
	}
	if n.EnvoyProxy == nil {
		n.EnvoyProxy = DefaultEnvoyProxy()
	}
}

func (n *NodeLocalLoadBalancing) Validate(path *field.Path) (errs field.ErrorList) {
	if n == nil {
		return
	}

	switch n.Type {
	case NllbTypeEnvoyProxy:
	case "":
		if n.IsEnabled() {
			errs = append(errs, field.Forbidden(path.Child("type"), "need to specify type if enabled"))
		}
	default:
		errs = append(errs, field.NotSupported(path.Child("type"), n.Type, []string{string(NllbTypeEnvoyProxy)}))
	}

	errs = append(errs, n.EnvoyProxy.Validate(path.Child("envoyProxy"))...)

	return
}

func (n *NodeLocalLoadBalancing) IsEnabled() bool {
	return n != nil && n.Enabled
}

// EnvoyProxy describes configuration options required for using Envoy as the
// backing implementation for node-local load balancing.
type EnvoyProxy struct {
	// image specifies the OCI image that's being used for the Envoy Pod.
	Image *ImageSpec `json:"image,omitempty"`

	// imagePullPolicy specifies the pull policy being used for the Envoy Pod.
	// Defaults to the default image pull policy.
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// apiServerBindPort is the port number on which to bind the Envoy load
	// balancer for the Kubernetes API server to on a worker's loopback
	// interface. This must be a valid port number, 0 < x < 65536.
	// Default: 7443
	// +kubebuilder:default=7443
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	APIServerBindPort int32 `json:"apiServerBindPort,omitempty"`

	// konnectivityServerBindPort is the port number on which to bind the Envoy
	// load balancer for the konnectivity server to on a worker's loopback
	// interface. This must be a valid port number, 0 < x < 65536.
	// Default: 7132
	// +kubebuilder:default=7132
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	KonnectivityServerBindPort *int32 `json:"konnectivityServerBindPort,omitempty"`
}

// DefaultEnvoyProxy returns the default envoy proxy configuration.
func DefaultEnvoyProxy() *EnvoyProxy {
	p := new(EnvoyProxy)
	p.setDefaults()
	return p
}

var _ json.Unmarshaler = (*EnvoyProxy)(nil)

func (p *EnvoyProxy) UnmarshalJSON(data []byte) error {
	type envoyProxy EnvoyProxy
	if err := json.Unmarshal(data, (*envoyProxy)(p)); err != nil {
		return err
	}

	p.setDefaults()

	return nil
}

func (p *EnvoyProxy) setDefaults() {
	if p.Image == nil {
		p.Image = DefaultEnvoyProxyImage()
	} else {
		if p.Image.Image == "" {
			p.Image.Image = constant.EnvoyProxyImage
		}
		if p.Image.Version == "" {
			p.Image.Version = constant.EnvoyProxyImageVersion
		}
	}
	if p.APIServerBindPort == 0 {
		p.APIServerBindPort = 7443
	}
	if p.KonnectivityServerBindPort == nil {
		p.KonnectivityServerBindPort = ptr.To(int32(7132))
	}
}

func (p *EnvoyProxy) Validate(path *field.Path) (errs field.ErrorList) {
	if p == nil {
		return
	}

	image := path.Child("image")
	if p.Image == nil {
		errs = append(errs, field.Required(image, "image must be set"))
	} else {
		errs = append(errs, p.Image.Validate(image)...)
	}

	switch p.ImagePullPolicy {
	case corev1.PullAlways, corev1.PullNever, corev1.PullIfNotPresent, "":
		break
	default:
		errs = append(errs, field.NotSupported(
			path.Child("imagePullPolicy"), p.ImagePullPolicy, []string{
				string(corev1.PullAlways),
				string(corev1.PullNever),
				string(corev1.PullIfNotPresent),
			},
		))
	}

	if details := validation.IsValidPortNum(int(p.APIServerBindPort)); len(details) > 0 {
		path := path.Child("apiServerBindPort")
		for _, detail := range details {
			errs = append(errs, field.Invalid(path, p.APIServerBindPort, detail))
		}
	}

	if p.KonnectivityServerBindPort != nil {
		if details := validation.IsValidPortNum(int(*p.KonnectivityServerBindPort)); len(details) > 0 {
			path := path.Child("konnectivityServerBindPort")
			for _, detail := range details {
				errs = append(errs, field.Invalid(path, p.KonnectivityServerBindPort, detail))
			}
		}
	}

	return
}

// DefaultEnvoyProxyImage returns the default image spec to use for Envoy.
func DefaultEnvoyProxyImage() *ImageSpec {
	return &ImageSpec{
		Image:   constant.EnvoyProxyImage,
		Version: constant.EnvoyProxyImageVersion,
	}
}
