// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Validateable = (*KubeProxy)(nil)

const (
	ModeIptables  = "iptables"
	ModeIPVS      = "ipvs"
	ModeUSerspace = "userspace"
	ModeNFT       = "nftables"
)

// KubeProxy defines the configuration for kube-proxy
type KubeProxy struct {
	Disabled bool `json:"disabled,omitempty"`
	// Mode defines the kube-proxy mode. Supported values are "iptables", "ipvs", "userspace" and "nft"
	// Defaults to "iptables"
	Mode               string `json:"mode,omitempty"`
	MetricsBindAddress string `json:"metricsBindAddress,omitempty"`
	// +optional
	IPTables KubeProxyIPTablesConfiguration `json:"iptables"`
	// +optional
	IPVS KubeProxyIPVSConfiguration `json:"ipvs"`
	// +optional
	NFTables          KubeProxyNFTablesConfiguration `json:"nftables"`
	NodePortAddresses []string                       `json:"nodePortAddresses,omitempty"`

	// Map of key-values (strings) for any extra arguments to pass down to kube-proxy process
	// Any behavior triggered by these parameters is outside k0s support.
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`

	// Slice of strings with raw arguments to pass to the kube-proxy process
	// These arguments will be appended to the `ExtraArgs` and aren't validated at all.
	// ExtraArgs are recommended over RawArgs. If possible use ExtraArgs to set arguments.
	RawArgs []string `json:"rawArgs,omitempty"`
}

// KubeProxyIPTablesConfiguration contains iptables-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.34.3/config/v1alpha1/types.go#L27-L48
type KubeProxyIPTablesConfiguration struct {
	MasqueradeBit      *int32 `json:"masqueradeBit,omitempty"`
	MasqueradeAll      bool   `json:"masqueradeAll,omitempty"`
	LocalhostNodePorts *bool  `json:"localhostNodePorts,omitempty"`
	// +optional
	SyncPeriod metav1.Duration `json:"syncPeriod"`
	// +optional
	MinSyncPeriod metav1.Duration `json:"minSyncPeriod"`
}

// KubeProxyIPVSConfiguration contains ipvs-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.34.3/config/v1alpha1/types.go#L52-L78
type KubeProxyIPVSConfiguration struct {
	// +optional
	SyncPeriod metav1.Duration `json:"syncPeriod"`
	// +optional
	MinSyncPeriod metav1.Duration `json:"minSyncPeriod"`
	Scheduler     string          `json:"scheduler,omitempty"`
	ExcludeCIDRs  []string        `json:"excludeCIDRs,omitempty"`
	StrictARP     bool            `json:"strictARP,omitempty"`
	// +optional
	TCPTimeout metav1.Duration `json:"tcpTimeout"`
	// +optional
	TCPFinTimeout metav1.Duration `json:"tcpFinTimeout"`
	// +optional
	UDPTimeout metav1.Duration `json:"udpTimeout"`
}

// KubeProxyNFTablesConfiguration contains nftables-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.34.3/config/v1alpha1/types.go#L82-L97
type KubeProxyNFTablesConfiguration struct {
	// +optional
	SyncPeriod    metav1.Duration `json:"syncPeriod"`
	MasqueradeBit *int32          `json:"masqueradeBit,omitempty"`
	MasqueradeAll bool            `json:"masqueradeAll,omitempty"`
	// +optional
	MinSyncPeriod metav1.Duration `json:"minSyncPeriod"`
}

// DefaultKubeProxy creates the default config for kube-proxy
func DefaultKubeProxy() *KubeProxy {
	return &KubeProxy{
		Mode:               ModeIptables,
		MetricsBindAddress: "0.0.0.0:10249",
	}
}

// Validate validates kube proxy config
func (k *KubeProxy) Validate() []error {
	if k.Disabled {
		return nil
	}
	var errors []error
	if k.Mode != ModeIptables && k.Mode != ModeIPVS && k.Mode != ModeUSerspace && k.Mode != ModeNFT {
		errors = append(errors, fmt.Errorf("unsupported mode %s for kubeProxy config", k.Mode))
	}
	return errors
}
