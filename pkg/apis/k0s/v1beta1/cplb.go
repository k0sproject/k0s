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
)

// Defaults are keepalived's defaults.
const defaultVirtualRouterID = 51
const defaultAdvertInterval = 1

// ControlPlaneLoadBalancingSpec defines the configuration options related to k0s's
// keepalived feature.
type ControlPlaneLoadBalancingSpec struct {
	// Indicates if control plane load balancing should be enabled.
	// Default: false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Configuration options related to the VRRP. This is an array which allows
	// to configure multiple virtual IPs.
	VRRPInstances VRRPInstances `json:"vrrpInstances,omitempty"`
}

// VRRPInstances is a list of VRRPInstance
type VRRPInstances []VRRPInstance

// VRRPInstance defines the configuration options for a VRRP instance.
type VRRPInstance struct {
	// Name is the name of the VRRP instance. If not specified, defaults to
	// k0s-vip-<index>.
	//+kubebuilder:default=k0s-vip
	Name string `json:"name,omitempty"`

	// VirtualIP is the list virtual IP address used by the VRRP instance. VirtualIPs
	// must be a CIDR as defined in RFC 4632 and RFC 4291.
	VirtualIPs VirtualIPs `json:"virtualIPs,omitempty"`

	// Interface specifies the NIC used by the virtual router. If not specified,
	// k0s will use the interface that owns the default route.
	Interface string `json:"interface,omitempty"`

	// VirtualRouterID is the VRRP router ID. If not specified, defaults to 51.
	// VirtualRouterID must be in the range of 1-255, all the control plane
	// nodes must have the same VirtualRouterID.
	// Two clusters in the same network must not use the same VirtualRouterID.
	//+kubebuilder:validation:Minimum=1
	//+kubebuilder:validation:Maximum=255
	//+kubebuilder:default=51
	VirtualRouterID *int32 `json:"virtualRouterID,omitempty"`

	// AdvertInterval is the advertisement interval in seconds. If not specified,
	// use 1 second
	//+kubebuilder:default=1
	AdvertInterval *int32 `json:"advertInterval,omitempty"`

	// AuthPass is the password for accessing vrrpd. This is not a security
	// feature but a way to prevent accidental misconfigurations.
	// Authpass must be 8 characters or less.
	AuthPass string `json:"authPass"`
}

type VirtualIPs []string

// ValidateVRRPInstances validates existing configuration and sets the default
// values of undefined fields.
func (c *ControlPlaneLoadBalancingSpec) ValidateVRRPInstances(getDefaultNICFn func() (string, error)) error {
	if getDefaultNICFn == nil {
		getDefaultNICFn = getDefaultNIC
	}
	for i, vi := range c.VRRPInstances {
		if vi.Name == "" {
			c.VRRPInstances[i].Name = fmt.Sprintf("k0s-vip-%d", i)
		}

		if vi.Interface == "" {
			nic, err := getDefaultNICFn()
			if err != nil {
				return fmt.Errorf("failed to get default NIC: %v", err)
			}
			c.VRRPInstances[i].Interface = nic
		}

		if vi.VirtualRouterID == nil {
			vrid := int32(defaultVirtualRouterID + i)
			c.VRRPInstances[i].VirtualRouterID = &vrid
		} else if *vi.VirtualRouterID < 0 || *vi.VirtualRouterID > 255 {
			return errors.New("VirtualRouterID must be in the range of 1-255")
		}

		if vi.AdvertInterval == nil {
			advInt := int32(defaultAdvertInterval)
			c.VRRPInstances[i].AdvertInterval = &advInt
		}

		if vi.AuthPass == "" {
			return errors.New("AuthPass must be defined")
		}
		if len(c.VRRPInstances[i].AuthPass) > 8 {
			return errors.New("AuthPass must be 8 characters or less")
		}

		if len(vi.VirtualIPs) == 0 {
			return errors.New("VirtualIPs must be defined")
		}
		for _, vip := range vi.VirtualIPs {
			if _, _, err := net.ParseCIDR(vip); err != nil {
				return fmt.Errorf("VirtualIPs must be a CIDR. Got: %s", vip)
			}
		}
	}
	return nil
}
