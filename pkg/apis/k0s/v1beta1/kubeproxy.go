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
	Mode               string                         `json:"mode,omitempty"`
	MetricsBindAddress string                         `json:"metricsBindAddress,omitempty"`
	IPTables           KubeProxyIPTablesConfiguration `json:"iptables,omitempty"`
	IPVS               KubeProxyIPVSConfiguration     `json:"ipvs,omitempty"`
	NFTables           KubeProxyNFTablesConfiguration `json:"nftables,omitempty"`
	NodePortAddresses  []string                       `json:"nodePortAddresses,omitempty"`

	// Map of key-values (strings) for any extra arguments to pass down to kube-proxy process
	// Any behavior triggered by these parameters is outside k0s support.
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// KubeProxyIPTablesConfiguration contains iptables-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.31.2/config/v1alpha1/types.go#L27-L48
type KubeProxyIPTablesConfiguration struct {
	MasqueradeBit      *int32          `json:"masqueradeBit,omitempty"`
	MasqueradeAll      bool            `json:"masqueradeAll,omitempty"`
	LocalhostNodePorts *bool           `json:"localhostNodePorts,omitempty"`
	SyncPeriod         metav1.Duration `json:"syncPeriod,omitempty"`
	MinSyncPeriod      metav1.Duration `json:"minSyncPeriod,omitempty"`
}

// KubeProxyIPVSConfiguration contains ipvs-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.31.2/config/v1alpha1/types.go#L52-L78
type KubeProxyIPVSConfiguration struct {
	SyncPeriod    metav1.Duration `json:"syncPeriod,omitempty"`
	MinSyncPeriod metav1.Duration `json:"minSyncPeriod,omitempty"`
	Scheduler     string          `json:"scheduler,omitempty"`
	ExcludeCIDRs  []string        `json:"excludeCIDRs,omitempty"`
	StrictARP     bool            `json:"strictARP,omitempty"`
	TCPTimeout    metav1.Duration `json:"tcpTimeout,omitempty"`
	TCPFinTimeout metav1.Duration `json:"tcpFinTimeout,omitempty"`
	UDPTimeout    metav1.Duration `json:"udpTimeout,omitempty"`
}

// KubeProxyNFTablesConfiguration contains nftables-related kube-proxy configuration
// @see https://github.com/kubernetes/kube-proxy/blob/v0.31.2/config/v1alpha1/types.go#L82-L97
type KubeProxyNFTablesConfiguration struct {
	SyncPeriod    metav1.Duration `json:"syncPeriod,omitempty"`
	MasqueradeBit *int32          `json:"masqueradeBit,omitempty"`
	MasqueradeAll bool            `json:"masqueradeAll,omitempty"`
	MinSyncPeriod metav1.Duration `json:"minSyncPeriod,omitempty"`
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
