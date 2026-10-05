// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"errors"
	"fmt"
	"math"
	"net"
	"slices"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

// Validates existing configuration and sets the default values of undefined fields.
func (i VRRPInstances) validate(path *field.Path, getDefaultNICFn func() (string, error)) (errs field.ErrorList) {
	if getDefaultNICFn == nil {
		getDefaultNICFn = sync.OnceValues(getDefaultNIC)
	}
	for j := range i {
		errs = append(errs, i[j].validate(path.Index(j), j, getDefaultNICFn)...)
	}
	return errs
}

func (i *VRRPInstance) validate(path *field.Path, offset int, getDefaultNICFn func() (string, error)) (errs field.ErrorList) {
	if i.Interface == "" {
		nic, err := getDefaultNICFn()
		if err != nil {
			errs = append(errs, field.InternalError(path.Child("interface"), fmt.Errorf("failed to get default NIC: %w", err)))
		}
		i.Interface = nic
	} else if _, err := net.ParseMAC(i.Interface); err == nil {
		if nic, err := getNIC(i.Interface); err != nil {
			errs = append(errs, field.InternalError(path.Child("interface"), fmt.Errorf("failed to get NIC for MAC address %s: %w", i.Interface, err)))
		} else {
			i.Interface = nic
		}
	}

	if id := i.VirtualRouterID; id == 0 {
		if id := defaultVirtualRouterID + int32(offset); id > 255 {
			errs = append(errs, field.InternalError(path.Child("virtualRouterID"), errors.New("automatic virtualRouterIDs exceeded, specify them explicitly")))
		} else {
			i.VirtualRouterID = id
		}
	} else if lo, hi := int32(1), int32(255); lo > id || id > hi {
		errs = append(errs, field.Invalid(path.Child("virtualRouterID"), id, fmt.Sprintf("must be between %d and %d, inclusive", lo, hi)))
	}

	switch i.AddressLabel {
	case 0:
		i.AddressLabel = 10000
	case math.MaxUint32:
		errs = append(errs, field.Invalid(path.Child("addressLabel"), i.AddressLabel, "0xffffffff is reserved"))
	}

	if i.AdvertIntervalSeconds == 0 {
		i.AdvertIntervalSeconds = defaultAdvertIntervalSeconds
	}

	if i.AuthPass == "" {
		errs = append(errs, field.Required(path.Child("authPass"), ""))
	} else if maxLen := 8; len(i.AuthPass) > maxLen {
		errs = append(errs, field.TooLong(path.Child("authPass"), i.AuthPass, maxLen))
	}

	if len(i.VirtualIPs) == 0 {
		errs = append(errs, field.Required(path.Child("virtualIPs"), ""))
	}
	for j, vip := range i.VirtualIPs {
		errs = append(errs, validation.IsValidInterfaceAddress(path.Child("virtualIPs").Index(j), vip)...)
	}

	if len(i.UnicastPeers) > 0 {
		if path, ip := path.Child("unicastSourceIP"), i.UnicastSourceIP; ip == "" {
			errs = append(errs, field.Required(path, "when unicastPeers are specified"))
		} else {
			errs = append(errs, validation.IsValidIP(path, ip)...)
		}
		for j, peer := range i.UnicastPeers {
			path := path.Child("unicastPeers").Index(j)
			errs = append(errs, validation.IsValidIP(path, peer)...)
			if peer != "" && peer == i.UnicastSourceIP {
				errs = append(errs, field.Invalid(path, peer, "must not be the same as unicastSourceIP"))
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

var validLBAlgos = [...]KeepalivedLBAlgo{
	RRAlgo,
	WRRAlgo,
	LCAlgo,
	WLCAlgo,
	LBLCAlgo,
	DHAlgo,
	SHAlgo,
	SEDAlgo,
	NQAlgo,
}

// KeepalivedLBKind describes the load balancing forwarding method.
// +kubebuilder:validation:Enum=NAT;DR;TUN
type KeepalivedLBKind string

const (
	NATLBKind KeepalivedLBKind = "NAT"
	DRLBKind  KeepalivedLBKind = "DR"
	TUNLBKind KeepalivedLBKind = "TUN"
)

// Validates existing configuration and sets the default values of undefined fields.
func (s VirtualServers) validate(path *field.Path) (errs field.ErrorList) {
	for i := range s {
		errs = append(errs, s[i].validate(path.Index(i))...)
	}
	return errs
}

var validLBKinds = [...]KeepalivedLBKind{
	NATLBKind,
	DRLBKind,
	TUNLBKind,
}

func (s *VirtualServer) validate(path *field.Path) (errs field.ErrorList) {
	if path, ip := path.Child("ipAddress"), s.IPAddress; ip == "" {
		errs = append(errs, field.Required(path, ""))
	} else {
		errs = append(errs, validation.IsValidIP(path, ip)...)
	}

	if s.LBAlgo == "" {
		s.LBAlgo = RRAlgo
	} else if !slices.Contains(validLBAlgos[:], s.LBAlgo) {
		errs = append(errs, field.NotSupported(path.Child("lbAlgo"), s.LBAlgo, validLBAlgos[:]))
	}

	if s.LBKind == "" {
		s.LBKind = DRLBKind
	} else if !slices.Contains(validLBKinds[:], s.LBKind) {
		errs = append(errs, field.NotSupported(path.Child("lbKind"), s.LBKind, validLBKinds[:]))
	}

	if s.PersistenceTimeoutSeconds == 0 {
		s.PersistenceTimeoutSeconds = 360
	} else {
		for _, msg := range validation.IsInRange(s.PersistenceTimeoutSeconds, 1, 2678400) {
			errs = append(errs, field.Invalid(path.Child("persistenceTimeoutSeconds"), s.PersistenceTimeoutSeconds, msg))
		}
	}

	if s.DelayLoop == (metav1.Duration{}) {
		s.DelayLoop = metav1.Duration{Duration: 1 * time.Minute}
	} else {
		s.DelayLoop.Duration = s.DelayLoop.Truncate(time.Microsecond)
		if s.DelayLoop.Duration <= 0 {
			errs = append(errs, field.Invalid(path.Child("delayLoop"), s.DelayLoop, "must be positive"))
		}
	}

	return errs
}

// Validate validates the ControlPlaneLoadBalancingSpec
func (c *ControlPlaneLoadBalancingSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if c == nil {
		return nil
	}

	switch c.Type {
	case CPLBTypeKeepalived:
	case "":
		c.Type = CPLBTypeKeepalived
	default:
		errs = append(errs, field.NotSupported(path.Child("type"), c.Type, []CPLBType{CPLBTypeKeepalived}))
	}

	return append(errs, c.Keepalived.Validate(path.Child("keepalived"))...)
}

// Validate validates the KeepalivedSpec
func (k *KeepalivedSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if k == nil {
		return nil
	}

	errs = append(errs, k.VRRPInstances.validate(path.Child("vrrpInstances"), nil)...)
	errs = append(errs, k.VirtualServers.validate(path.Child("virtualServers"))...)
	if k.UserSpaceProxyPort == 0 {
		k.UserSpaceProxyPort = 6444
	} else {
		for _, msg := range validation.IsValidPortNum(k.UserSpaceProxyPort) {
			errs = append(errs, field.Invalid(path.Child("userSpaceProxyBindPort"), k.UserSpaceProxyPort, msg))
		}
	}

	return errs
}
