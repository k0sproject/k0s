// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"net"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// inconsistentPodCIDROrderError is returned when the existing nodes' dual-stack
// pod CIDRs don't share a common family order. Starting kube-controller-manager
// in that state would crash it with confusing "out the range of cluster cidr"
// errors, so we refuse to start it instead.
type inconsistentPodCIDROrderError struct {
	node     string
	expected v1beta1.PrimaryAddressFamilyType
	actual   v1beta1.PrimaryAddressFamilyType
}

func (e *inconsistentPodCIDROrderError) Error() string {
	return fmt.Sprintf("node %q has %s-first pod CIDRs, but other nodes have %s-first pod CIDRs", e.node, e.actual, e.expected)
}

// podCIDRForCluster returns the value to pass to kube-controller-manager's
// --cluster-cidr flag. For dual-stack clusters it derives the pod CIDR family
// order from the existing nodes, so that clusters bootstrapped before k0s 1.36
// (which unconditionally allocated IPv6-first pod CIDRs, see
// https://github.com/k0sproject/k0s/issues/7927) keep working after an upgrade.
// New clusters without any dual-stack nodes yet fall back to the configured
// primary address family.
//
// A returned error is either a transient failure while talking to the API
// server, or an [*inconsistentPodCIDROrderError] in case the nodes disagree on
// the pod CIDR family order.
func podCIDRForCluster(ctx context.Context, factory kubeutil.ClientFactoryInterface, network *v1beta1.Network, primaryFamily v1beta1.PrimaryAddressFamilyType) (string, error) {
	if !network.DualStack.Enabled {
		return network.BuildPodCIDR(primaryFamily), nil
	}

	client, err := factory.GetClient()
	if err != nil {
		return "", fmt.Errorf("failed to get kube client: %w", err)
	}

	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	effectiveFamily, found, err := detectLegacyPodCIDROrder(nodes.Items)
	if err != nil {
		return "", err
	}
	if !found {
		effectiveFamily = primaryFamily
	}

	return network.BuildPodCIDR(effectiveFamily), nil
}

// detectLegacyPodCIDROrder inspects the existing nodes' allocated pod CIDRs
// and reports their family order. Only dual-stack nodes with two pod CIDRs are
// taken into account; single-stack nodes are ignored, as they don't carry any
// ordering information.
//
// The first return value is the family of the first pod CIDR, which determines
// the order in which kube-controller-manager must be started. found is false
// when there is no dual-stack node to derive an order from. An error is
// returned when the nodes disagree on the order.
func detectLegacyPodCIDROrder(nodes []corev1.Node) (v1beta1.PrimaryAddressFamilyType, bool, error) {
	var effectiveFamily v1beta1.PrimaryAddressFamilyType
	found := false

	for i := range nodes {
		node := &nodes[i]
		if len(node.Spec.PodCIDRs) != 2 {
			continue
		}

		_, firstCIDR, err := net.ParseCIDR(node.Spec.PodCIDRs[0])
		if err != nil {
			return "", false, fmt.Errorf("failed to parse pod CIDR %q of node %q: %w", node.Spec.PodCIDRs[0], node.Name, err)
		}

		var family v1beta1.PrimaryAddressFamilyType
		if firstCIDR.IP.To4() != nil {
			family = v1beta1.PrimaryFamilyIPv4
		} else {
			family = v1beta1.PrimaryFamilyIPv6
		}

		if !found {
			effectiveFamily = family
			found = true
			continue
		}
		if family != effectiveFamily {
			return "", false, &inconsistentPodCIDROrderError{node: node.Name, expected: effectiveFamily, actual: family}
		}
	}

	return effectiveFamily, found, nil
}
