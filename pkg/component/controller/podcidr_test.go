// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dualStackNode(name string, podCIDRs ...string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.NodeSpec{
			PodCIDRs: podCIDRs,
		},
	}
}

func TestDetectLegacyPodCIDROrder(t *testing.T) {
	t.Run("no nodes", func(t *testing.T) {
		_, found, err := detectLegacyPodCIDROrder(nil)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("single-stack nodes only", func(t *testing.T) {
		nodes := []corev1.Node{
			dualStackNode("node-1", "10.233.0.0/24"),
			dualStackNode("node-2", "fd00::/108"),
		}
		_, found, err := detectLegacyPodCIDROrder(nodes)
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("IPv6-first dual-stack nodes", func(t *testing.T) {
		nodes := []corev1.Node{
			dualStackNode("node-1", "fd00:172:16::1000/117", "172.16.0.0/24"),
			dualStackNode("node-2", "fd00:172:16::2000/117", "172.16.1.0/24"),
		}
		family, found, err := detectLegacyPodCIDROrder(nodes)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, v1beta1.PrimaryFamilyIPv6, family)
	})

	t.Run("IPv4-first dual-stack nodes", func(t *testing.T) {
		nodes := []corev1.Node{
			dualStackNode("node-1", "172.16.0.0/24", "fd00:172:16::1000/117"),
			dualStackNode("node-2", "172.16.1.0/24", "fd00:172:16::2000/117"),
		}
		family, found, err := detectLegacyPodCIDROrder(nodes)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, v1beta1.PrimaryFamilyIPv4, family)
	})

	t.Run("mixed order", func(t *testing.T) {
		nodes := []corev1.Node{
			dualStackNode("node-1", "fd00:172:16::1000/117", "172.16.0.0/24"),
			dualStackNode("node-2", "172.16.1.0/24", "fd00:172:16::2000/117"),
		}
		_, found, err := detectLegacyPodCIDROrder(nodes)
		assert.False(t, found)
		var inconsistent *inconsistentPodCIDROrderError
		require.ErrorAs(t, err, &inconsistent)
		assert.Equal(t, "node-2", inconsistent.node)
	})

	t.Run("single-stack nodes are ignored", func(t *testing.T) {
		nodes := []corev1.Node{
			dualStackNode("node-1", "10.233.0.0/24"),
			dualStackNode("node-2", "fd00:172:16::2000/117", "172.16.1.0/24"),
		}
		family, found, err := detectLegacyPodCIDROrder(nodes)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, v1beta1.PrimaryFamilyIPv6, family)
	})

	t.Run("invalid pod CIDR", func(t *testing.T) {
		nodes := []corev1.Node{dualStackNode("node-1", "not-a-cidr", "172.16.0.0/24")}
		_, found, err := detectLegacyPodCIDROrder(nodes)
		assert.False(t, found)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse pod CIDR")
	})
}

func TestPodCIDRForCluster(t *testing.T) {
	newDualStackNetwork := func() *v1beta1.Network {
		return &v1beta1.Network{
			PodCIDR:              "172.16.0.0/16",
			ServiceCIDR:          "172.31.0.0/16",
			DualStack:            v1beta1.DualStack{Enabled: true, IPv6PodCIDR: "fd00:172:16::/108", IPv6ServiceCIDR: "fd00:172:31::/108"},
			Provider:             "kuberouter",
			KubeRouter:           v1beta1.DefaultKubeRouter(),
			KubeProxy:            v1beta1.DefaultKubeProxy(),
			PrimaryAddressFamily: v1beta1.PrimaryFamilyUnknown,
		}
	}

	ctx := context.Background()

	t.Run("single-stack ignores nodes", func(t *testing.T) {
		network := newDualStackNetwork()
		network.DualStack.Enabled = false
		factory := testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"172.16.0.0/24"}}},
		)
		cidr, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv4)
		require.NoError(t, err)
		assert.Equal(t, "172.16.0.0/16", cidr)
	})

	t.Run("new cluster without nodes uses primary family", func(t *testing.T) {
		network := newDualStackNetwork()
		factory := testutil.NewFakeClientFactory()
		cidr, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv4)
		require.NoError(t, err)
		assert.Equal(t, "172.16.0.0/16,fd00:172:16::/108", cidr)
	})

	t.Run("legacy IPv6-first nodes are preserved", func(t *testing.T) {
		network := newDualStackNetwork()
		factory := testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"fd00:172:16::1000/117", "172.16.0.0/24"}}},
		)
		cidr, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv4)
		require.NoError(t, err)
		assert.Equal(t, "fd00:172:16::/108,172.16.0.0/16", cidr)
	})

	t.Run("IPv4-first nodes stay IPv4-first", func(t *testing.T) {
		network := newDualStackNetwork()
		factory := testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"172.16.0.0/24", "fd00:172:16::1000/117"}}},
		)
		cidr, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv4)
		require.NoError(t, err)
		assert.Equal(t, "172.16.0.0/16,fd00:172:16::/108", cidr)
	})

	t.Run("inconsistent node order is an error", func(t *testing.T) {
		network := newDualStackNetwork()
		factory := testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"fd00:172:16::1000/117", "172.16.0.0/24"}}},
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"172.16.1.0/24", "fd00:172:16::2000/117"}}},
		)
		_, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv4)
		var inconsistent *inconsistentPodCIDROrderError
		require.ErrorAs(t, err, &inconsistent)
	})

	t.Run("IPv6 primary family", func(t *testing.T) {
		network := newDualStackNetwork()
		factory := testutil.NewFakeClientFactory()
		cidr, err := podCIDRForCluster(ctx, factory, network, v1beta1.PrimaryFamilyIPv6)
		require.NoError(t, err)
		assert.Equal(t, "fd00:172:16::/108,172.16.0.0/16", cidr)
	})
}
