// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func dualStackClusterConfig() *v1beta1.ClusterConfig {
	network := &v1beta1.Network{
		PodCIDR:              "172.16.0.0/16",
		ServiceCIDR:          "172.31.0.0/16",
		DualStack:            v1beta1.DualStack{Enabled: true, IPv6PodCIDR: "fd00:172:16::/108", IPv6ServiceCIDR: "fd00:172:31::/108"},
		Provider:             "kuberouter",
		KubeRouter:           v1beta1.DefaultKubeRouter(),
		KubeProxy:            v1beta1.DefaultKubeProxy(),
		PrimaryAddressFamily: v1beta1.PrimaryFamilyUnknown,
	}
	return &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			ControllerManager: &v1beta1.ControllerManagerSpec{},
			Network:           network,
		},
	}
}

func newManagerForTest() *Manager {
	return &Manager{
		K0sVars: &config.CfgVars{
			CertRootDir: "/var/lib/k0s/pki",
		},
		ServiceClusterIPRange: "172.31.0.0/16,fd00:172:31::/108",
		PrimaryAddressFamily:  v1beta1.PrimaryFamilyIPv4,
	}
}

func TestManagerBuildArgs(t *testing.T) {
	ctx := context.Background()

	t.Run("single-stack cluster", func(t *testing.T) {
		clusterConfig := dualStackClusterConfig()
		clusterConfig.Spec.Network.DualStack.Enabled = false
		manager := newManagerForTest()

		args, err := manager.buildArgs(ctx, clusterConfig)
		require.NoError(t, err)
		assert.Equal(t, "172.16.0.0/16", args["cluster-cidr"])
	})

	t.Run("dual-stack new cluster uses primary family", func(t *testing.T) {
		clusterConfig := dualStackClusterConfig()
		manager := newManagerForTest()
		manager.KubeClientFactory = testutil.NewFakeClientFactory()

		args, err := manager.buildArgs(ctx, clusterConfig)
		require.NoError(t, err)
		assert.Equal(t, "172.16.0.0/16,fd00:172:16::/108", args["cluster-cidr"])
	})

	t.Run("dual-stack legacy IPv6-first nodes are preserved", func(t *testing.T) {
		clusterConfig := dualStackClusterConfig()
		manager := newManagerForTest()
		manager.KubeClientFactory = testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"fd00:172:16::1000/117", "172.16.0.0/24"}}},
		)

		args, err := manager.buildArgs(ctx, clusterConfig)
		require.NoError(t, err)
		assert.Equal(t, "fd00:172:16::/108,172.16.0.0/16", args["cluster-cidr"])
	})

	t.Run("explicit cluster-cidr extra arg always wins", func(t *testing.T) {
		clusterConfig := dualStackClusterConfig()
		clusterConfig.Spec.ControllerManager.ExtraArgs = map[string]string{
			"cluster-cidr": "fd00:172:16::/108,172.16.0.0/16",
		}
		manager := newManagerForTest()
		manager.KubeClientFactory = testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"172.16.0.0/24", "fd00:172:16::1000/117"}}},
		)

		args, err := manager.buildArgs(ctx, clusterConfig)
		require.NoError(t, err)
		assert.Equal(t, "fd00:172:16::/108,172.16.0.0/16", args["cluster-cidr"])
	})

	t.Run("inconsistent node order fails reconciliation", func(t *testing.T) {
		clusterConfig := dualStackClusterConfig()
		manager := newManagerForTest()
		manager.KubeClientFactory = testutil.NewFakeClientFactory(
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"fd00:172:16::1000/117", "172.16.0.0/24"}}},
			&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Spec: corev1.NodeSpec{PodCIDRs: []string{"172.16.1.0/24", "fd00:172:16::2000/117"}}},
		)

		_, err := manager.buildArgs(ctx, clusterConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "refusing to start kube-controller-manager")
	})
}
