// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"fmt"
	"math"
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
	// UserspaceProxyPort is the port where the userspace proxy will bind
	// to. This port is only used internally, but listens on every interface.
	// Defaults to 6444
	//
	// +kubebuilder:default=6444
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	UserSpaceProxyPort int `json:"userSpaceProxyBindPort,omitempty"`
	// DisableLoadBalancer disables the load balancer.
	DisableLoadBalancer bool `json:"disableLoadBalancer,omitempty"`
	// ConfigTemplateVRRP specifies the path to a custom Keepalived configuration template for VRRP.
	// If specified, this template will be used instead of the default configuration.
	// The template must be a valid Go template and will receive keepalivedConfig as input.
	// +optional
	ConfigTemplateVRRP string `json:"configTemplateVRRP,omitempty"`
	// ConfigTemplateVS specifies the path to a custom Keepalived configuration template for Virtual Servers.
	// If specified, this template will be used instead of the default configuration.
	// The template must be a valid Go template and will receive keepalivedConfig as input.
	// +optional
	ConfigTemplateVS string `json:"configTemplateVS,omitempty"`
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

	// Interface specifies the NIC used by the virtual router.
	// If not specified, k0s will use the interface that owns the default route.
	// If a MAC address is specified instead of an interface name, k0s will
	// try to resolve the interface name based on the MAC address.
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

	// UnicastPeers is a list of unicast peers. If not specified, k0s will use multicast.
	// If specified, UnicastSourceIP must be specified as well.
	// +listType=set
	UnicastPeers []string `json:"unicastPeers,omitempty"`

	// UnicastSourceIP is the source address for unicast peers.
	// If not specified, k0s will use the first address of the interface.
	UnicastSourceIP string `json:"unicastSourceIP,omitempty"`

	// AddressLabel is label for the VRRP instance for IPv6 VIPs.
	// This value is ignored for IPv4 VIPs. This is used to set the routing preference
	// as per RFC 6724.
	// The value must be in the range from 1 to 2^32-1.
	// If not specificied or set to 0, defaults to 10000.
	// +kubebuilder:default=10000
	AddressLabel uint32 `json:"addressLabel,omitempty"`
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
		} else if _, err := net.ParseMAC(k.VRRPInstances[i].Interface); err == nil {
			macToInterfaceName(&k.VRRPInstances[i].Interface, &errs)
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

		if k.VRRPInstances[i].AddressLabel == 0 {
			k.VRRPInstances[i].AddressLabel = 10000
		}
		if k.VRRPInstances[i].AddressLabel == math.MaxUint32 {
			errs = append(errs, errors.New("AddressLabel 0xffffffff is reserved"))
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

		if len(k.VRRPInstances[i].UnicastPeers) > 0 {
			if net.ParseIP(k.VRRPInstances[i].UnicastSourceIP) == nil {
				errs = append(errs, fmt.Errorf("UnicastPeers require a valid UnicastSourceIP. Got: %s", k.VRRPInstances[i].UnicastSourceIP))
			}
			for _, peer := range k.VRRPInstances[i].UnicastPeers {
				if net.ParseIP(peer) == nil {
					errs = append(errs, fmt.Errorf("UnicastPeers require valid IP addresses. Got: %s", peer))
				}
				if peer == k.VRRPInstances[i].UnicastSourceIP {
					errs = append(errs, fmt.Errorf("UnicastPeers must not contain the UnicastSourceIP. Got: %s", peer))
				}
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
	//
	// +kubebuilder:default="1m"
	// +optional
	DelayLoop metav1.Duration `json:"delayLoop"`
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
func (c *ControlPlaneLoadBalancingSpec) Validate() (errs []error) {
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

	return append(errs, c.Keepalived.Validate()...)
}

// Validate validates the KeepalivedSpec
func (k *KeepalivedSpec) Validate() (errs []error) {
	if k == nil {
		return nil
	}

	errs = append(errs, k.validateVRRPInstances(nil)...)
	errs = append(errs, k.validateVirtualServers()...)
	if k.UserSpaceProxyPort == 0 {
		k.UserSpaceProxyPort = 6444
	} else if k.UserSpaceProxyPort < 1 || k.UserSpaceProxyPort > 65535 {
		errs = append(errs, errors.New("UserSpaceProxyPort must be in the range of 1-65535"))
	}

	return errs
}
