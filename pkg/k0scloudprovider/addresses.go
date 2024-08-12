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

package k0scloudprovider

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

const (
	ExternalIPAnnotation = "k0sproject.io/node-ip-external"
)

// AddressCollector finds addresses on a node.
type AddressCollector func(node *v1.Node) []v1.NodeAddress

// DefaultAddressCollector finds all of the internal and external IP addresses defined on
// the provided node.
func DefaultAddressCollector() AddressCollector {
	return func(node *v1.Node) []v1.NodeAddress {
		if node == nil {
			return []v1.NodeAddress{}
		}

		addresses := make([]v1.NodeAddress, 0)

		populateInternalAddress(&addresses, node)
		populateExternalAddress(&addresses, node)

		return addresses
	}
}

// populateInternalAddress finds the current "InternalIP" address for the provided node using
// a number of interrogation methods (provided annotations via --node-ip), as well as leveraging
// the last status.
func populateInternalAddress(addrs *[]v1.NodeAddress, node *v1.Node) {
	if addrs == nil || node == nil {
		return
	}

	// In scenarios where `--node-ip=<ip addr>` is provided to kubelet, a special annotation will be
	// added to the node indicating the "provided" IP address.
	//
	// This needs to be provided as `node_controller.go` asserts on this.

	if providedInternalIP, ok := node.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
		*addrs = append(*addrs, v1.NodeAddress{Type: v1.NodeInternalIP, Address: providedInternalIP})
		return
	}

	// Finally, if no k0s or node-ip adornments on the node have been found, rely on the IP addresses that
	// have already been reported in status.

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			*addrs = append(*addrs, addr)
		}
	}
}

// populateExternalAddress finds the current "ExternalIP" address as defined by the special
// k0s label on the provided node.
func populateExternalAddress(addrs *[]v1.NodeAddress, node *v1.Node) {
	if addrs == nil || node == nil {
		return
	}

	// Search the nodes annotations for any external IP address definitions.
	if externalIP, ok := node.Annotations[ExternalIPAnnotation]; ok {
		for _, addr := range strings.Split(externalIP, ",") {
			*addrs = append(*addrs, v1.NodeAddress{Type: v1.NodeExternalIP, Address: addr})
		}
	}
}
