/*
Copyright 2024 k0s authors

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
	"errors"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Defaults are keepalived's defaults.
const (
	defaultVirtualRouterID       = 51
	defaultAdvertIntervalSeconds = 1
)

// ControlPlaneLoadBalancingSpec defines the configuration options related to k0s's
// keepalived feature.
type ControlPlaneLoadBalancingSpec struct {
	// Indicates if control plane load balancing should be enabled.
	// Default: false
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled"`

	// type indicates the type of the control plane load balancer to deploy on
	// controller nodes. Currently, the only supported type is "Keepalived".
	// +kubebuilder:default=Keepalived
	Type CPLBType `json:"type,omitempty"`

	// Keepalived contains configuration options related to the "Keepalived" type
	// of load balancing.
	Keepalived *KeepalivedSpec `json:"keepalived,omitempty"`
}

// CPLBType describes which type of load balancer should be deployed for the
// control plane load balancing. The default is [CPLBTypeKeepalived].
// +kubebuilder:validation:Enum=Keepalived
type CPLBType string

const (
	// CPLBTypeKeepalived selects Keepalived as the backing load balancer.
	CPLBTypeKeepalived CPLBType = "Keepalived"
)

type KeepalivedSpec struct {
	// Configuration options related to the VRRP. This is an array which allows
	// to configure multiple virtual IPs.
	VRRPInstances VRRPInstances `json:"vrrpInstances,omitempty"`
	// Configuration options related to the virtual servers. This is an array
	// which allows to configure multiple load balancers.
	VirtualServers VirtualServers `json:"virtualServers,omitempty"`
}

// VRRPInstances is a list of VRRPInstance
// +kubebuilder:validation:MaxItems=255
type VRRPInstances []VRRPInstance

// VRRPInstance defines the configuration options for a VRRP instance.
type VRRPInstance struct {
	// VirtualIPs is the list of virtual IP address used by the VRRP instance.
	// Each virtual IP must be a CIDR as defined in RFC 4632 and RFC 4291.
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	VirtualIPs []string `json:"virtualIPs"`

	// Interface specifies the NIC used by the virtual router. If not specified,
	// k0s will use the interface that owns the default route.
	Interface string `json:"interface,omitempty"`

	// VirtualRouterID is the VRRP router ID. If not specified, k0s will
	// automatically number the IDs for each VRRP instance, starting with 51.
	// VirtualRouterID must be in the range of 1-255, all the control plane
	// nodes must use the same VirtualRouterID. Other clusters in the same
	// network must not use the same VirtualRouterID.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=255
	VirtualRouterID int32 `json:"virtualRouterID,omitempty"`

	// AdvertIntervalSeconds is the advertisement interval in seconds. Defaults to 1
	// second.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	AdvertIntervalSeconds int32 `json:"advertIntervalSeconds,omitempty"`

	// AuthPass is the password for accessing VRRPD. This is not a security
	// feature but a way to prevent accidental misconfigurations.
	// AuthPass must be 8 characters or less.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=8
	AuthPass string `json:"authPass"`
}

// validateVRRPInstances validates existing configuration and sets the default
// values of undefined fields.
func (k *KeepalivedSpec) validateVRRPInstances(getDefaultNICFn func() (string, error)) []error {
	errs := []error{}
	if getDefaultNICFn == nil {
		getDefaultNICFn = getDefaultNIC
	}
	for i := range k.VRRPInstances {
		if k.VRRPInstances[i].Interface == "" {
			nic, err := getDefaultNICFn()
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get default NIC: %w", err))
			}
			k.VRRPInstances[i].Interface = nic
		}

		if k.VRRPInstances[i].VirtualRouterID == 0 {
			id := defaultVirtualRouterID + int32(i)
			if id > 255 {
				errs = append(errs, errors.New("automatic virtualRouterIDs exceeded, specify them explicitly"))
			}
			k.VRRPInstances[i].VirtualRouterID = defaultVirtualRouterID + int32(i)
		} else if k.VRRPInstances[i].VirtualRouterID < 0 || k.VRRPInstances[i].VirtualRouterID > 255 {
			errs = append(errs, errors.New("VirtualRouterID must be in the range of 1-255"))
		}

		if k.VRRPInstances[i].AdvertIntervalSeconds == 0 {
			k.VRRPInstances[i].AdvertIntervalSeconds = defaultAdvertIntervalSeconds
		}

		if k.VRRPInstances[i].AuthPass == "" {
			errs = append(errs, errors.New("AuthPass must be defined"))
		}
		if len(k.VRRPInstances[i].AuthPass) > 8 {
			errs = append(errs, errors.New("AuthPass must be 8 characters or less"))
		}

		if len(k.VRRPInstances[i].VirtualIPs) == 0 {
			errs = append(errs, errors.New("VirtualIPs must be defined"))
		}
		for _, vip := range k.VRRPInstances[i].VirtualIPs {
			if _, _, err := net.ParseCIDR(vip); err != nil {
				errs = append(errs, fmt.Errorf("VirtualIPs must be a CIDR. Got: %s", vip))
			}
		}
	}
	return errs
}

// VirtualServers is a list of VirtualServer
// +listType=map
// +listMapKey=ipAddress
type VirtualServers []VirtualServer

// VirtualServer defines the configuration options for a virtual server.
type VirtualServer struct {
	// IPAddress is the virtual IP address used by the virtual server.
	// +kubebuilder:validation:MinLength=1
	IPAddress string `json:"ipAddress"`
	// DelayLoop is the delay timer for check polling. DelayLoop accepts
	// microsecond precision. Further precision will be truncated without
	// warnings. Defaults to 1m.
	// +kubebuilder:default="1m"
	DelayLoop metav1.Duration `json:"delayLoop,omitempty"`
	// LBAlgo is the load balancing algorithm. If not specified, defaults to rr.
	// Valid values are rr, wrr, lc, wlc, lblc, dh, sh, sed, nq. For further
	// details refer to keepalived documentation.
	// +kubebuilder:default=rr
	LBAlgo KeepalivedLBAlgo `json:"lbAlgo,omitempty"`
	// LBKind is the load balancing kind. If not specified, defaults to DR.
	// Valid values are NAT DR TUN. For further details refer to keepalived documentation.
	// +kubebuilder:default=DR
	LBKind KeepalivedLBKind `json:"lbKind,omitempty"`
	// PersistenceTimeoutSeconds specifies a timeout value for persistent
	// connections in seconds. PersistentTimeoutSeconds must be in the range of
	// 1-2678400 (31 days). If not specified, defaults to 360 (6 minutes).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=2678400
	// +kubebuilder:default=360
	PersistenceTimeoutSeconds int `json:"persistenceTimeoutSeconds,omitempty"`
}

// KeepalivedLBAlgo describes the load balancing algorithm.
// +kubebuilder:validation:Enum=rr;wrr;lc;wlc;lblc;dh;sh;sed;nq
type KeepalivedLBAlgo string

const (
	RRAlgo   KeepalivedLBAlgo = "rr"
	WRRAlgo  KeepalivedLBAlgo = "wrr"
	LCAlgo   KeepalivedLBAlgo = "lc"
	WLCAlgo  KeepalivedLBAlgo = "wlc"
	LBLCAlgo KeepalivedLBAlgo = "lblc"
	DHAlgo   KeepalivedLBAlgo = "dh"
	SHAlgo   KeepalivedLBAlgo = "sh"
	SEDAlgo  KeepalivedLBAlgo = "sed"
	NQAlgo   KeepalivedLBAlgo = "nq"
)

// KeepalivedLBKind describes the load balancing forwarding method.
// +kubebuilder:validation:Enum=NAT;DR;TUN
type KeepalivedLBKind string

const (
	NATLBKind KeepalivedLBKind = "NAT"
	DRLBKind  KeepalivedLBKind = "DR"
	TUNLBKind KeepalivedLBKind = "TUN"
)

// validateVirtualServers validates existing configuration and sets the default
// values of undefined fields.
func (k *KeepalivedSpec) validateVirtualServers() []error {
	errs := []error{}
	for i := range k.VirtualServers {
		if k.VirtualServers[i].IPAddress == "" {
			errs = append(errs, errors.New("IPAddress must be defined"))
		}
		if net.ParseIP(k.VirtualServers[i].IPAddress) == nil {
			errs = append(errs, fmt.Errorf("invalid IP address: %s", k.VirtualServers[i].IPAddress))
		}

		if k.VirtualServers[i].LBAlgo == "" {
			k.VirtualServers[i].LBAlgo = RRAlgo
		} else {
			switch k.VirtualServers[i].LBAlgo {
			case RRAlgo, WRRAlgo, LCAlgo, WLCAlgo, LBLCAlgo, DHAlgo, SHAlgo, SEDAlgo, NQAlgo:
				// valid LBAlgo
			default:
				errs = append(errs, fmt.Errorf("invalid LBAlgo: %s ", k.VirtualServers[i].LBAlgo))
			}
		}

		if k.VirtualServers[i].LBKind == "" {
			k.VirtualServers[i].LBKind = DRLBKind
		} else {
			switch k.VirtualServers[i].LBKind {
			case NATLBKind, DRLBKind, TUNLBKind:
				// valid LBKind
			default:
				errs = append(errs, fmt.Errorf("invalid LBKind: %s ", k.VirtualServers[i].LBKind))
			}
		}

		if k.VirtualServers[i].PersistenceTimeoutSeconds == 0 {
			k.VirtualServers[i].PersistenceTimeoutSeconds = 360
		} else if k.VirtualServers[i].PersistenceTimeoutSeconds < 1 || k.VirtualServers[i].PersistenceTimeoutSeconds > 2678400 {
			errs = append(errs, errors.New("PersistenceTimeout must be in the range of 1-2678400"))
		}

		if k.VirtualServers[i].DelayLoop == (metav1.Duration{}) {
			k.VirtualServers[i].DelayLoop = metav1.Duration{Duration: 1 * time.Minute}
		} else {
			k.VirtualServers[i].DelayLoop.Duration = k.VirtualServers[i].DelayLoop.Truncate(time.Microsecond)
			if k.VirtualServers[i].DelayLoop.Microseconds() <= 0 {
				errs = append(errs, errors.New("DelayLoop must be positive"))
			}
		}
	}
	return errs
}

// Validate validates the ControlPlaneLoadBalancingSpec
func (c *ControlPlaneLoadBalancingSpec) Validate(externalAddress string) (errs []error) {
	if c == nil {
		return nil
	}

	switch c.Type {
	case CPLBTypeKeepalived:
	case "":
		c.Type = CPLBTypeKeepalived
	default:
		errs = append(errs, fmt.Errorf("unsupported CPLB type: %s. Only allowed value: %s", c.Type, CPLBTypeKeepalived))
	}

	return append(errs, c.Keepalived.Validate(externalAddress)...)
}

// Validate validates the KeepalivedSpec
func (k *KeepalivedSpec) Validate(externalAddress string) (errs []error) {
	if k == nil {
		return nil
	}

	errs = append(errs, k.validateVRRPInstances(nil)...)
	errs = append(errs, k.validateVirtualServers()...)
	// CPLB reconciler relies in watching kubernetes.default.svc endpoints
	if externalAddress != "" && len(k.VirtualServers) > 0 {
		errs = append(errs, errors.New(".spec.api.externalAddress and virtual servers cannot be used together"))
	}

	return errs
}
