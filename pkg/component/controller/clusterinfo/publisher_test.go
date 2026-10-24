// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPublisher(t *testing.T, objects ...runtime.Object) *Publisher {
	t.Helper()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(k0sVars.CertRootDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(k0sVars.CertRootDir, "ca.crt"), caPEM(t), 0o600))
	nodeConfig := v1beta1.DefaultClusterConfig()
	nodeConfig.Spec.API.Address = "10.0.0.1"
	nodeConfig.Spec.API.ClusterInfoDiscovery = true
	p := &Publisher{K0sVars: k0sVars, NodeConfig: nodeConfig, Clients: testutil.NewFakeClientFactory(objects...)}
	require.NoError(t, p.Init(t.Context()))
	return p
}

// manifestObject returns the object of the given kind and name from the manifest.
func manifestObject(t *testing.T, path, kind, name string) *unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	resources, err := applier.ReadUnstructuredStream(bytes.NewReader(data), path)
	require.NoError(t, err)
	for _, r := range resources {
		if r.GetKind() == kind && r.GetName() == name {
			return r
		}
	}
	require.Failf(t, "object not found", "%s/%s in %s", kind, name, path)
	return nil
}

func TestPublisher_Sync(t *testing.T) {
	p := newPublisher(t)
	manifest := filepath.Join(p.manifestDir, "cluster-info.yaml")

	require.NoError(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
	clusterInfo := manifestObject(t, manifest, "ConfigMap", "cluster-info")
	kubeconfig, _, err := unstructured.NestedString(clusterInfo.Object, "data", "kubeconfig")
	require.NoError(t, err)
	assert.Contains(t, kubeconfig, "server: https://10.0.0.1:6443")

	p.NodeConfig.Spec.API.ClusterInfoDiscovery = false
	require.NoError(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
	assert.NoDirExists(t, p.manifestDir, "everything is pruned once disabled")
}

func TestPublisher_NoStackWhileDisabled(t *testing.T) {
	p := newPublisher(t)
	p.NodeConfig.Spec.API.ClusterInfoDiscovery = false
	require.NoError(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
	assert.NoDirExists(t, p.manifestDir, "other clusters get no stack")
}

func TestPublisher_SyncWithControlPlaneLoadBalancing(t *testing.T) {
	p := newPublisher(t)
	p.NodeConfig.Spec.Network.ControlPlaneLoadBalancing = &v1beta1.ControlPlaneLoadBalancingSpec{
		Enabled: true, Type: v1beta1.CPLBTypeKeepalived,
		Keepalived: &v1beta1.KeepalivedSpec{VRRPInstances: v1beta1.VRRPInstances{{VirtualIPs: []string{"10.0.0.100/24"}}}},
	}
	require.NoError(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))

	// Joining nodes use the virtual IP, which moves on failover.
	clusterInfo := manifestObject(t, filepath.Join(p.manifestDir, "cluster-info.yaml"), "ConfigMap", "cluster-info")
	kubeconfig, _, err := unstructured.NestedString(clusterInfo.Object, "data", "kubeconfig")
	require.NoError(t, err)
	assert.Contains(t, kubeconfig, "server: https://10.0.0.100:6443")
}

func TestPublisher_SyncErrors(t *testing.T) {
	t.Run("no CA certificate", func(t *testing.T) {
		p := newPublisher(t)
		require.NoError(t, os.Remove(filepath.Join(p.K0sVars.CertRootDir, "ca.crt")))
		assert.ErrorIs(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()), os.ErrNotExist)
	})

	t.Run("invalid service CIDR", func(t *testing.T) {
		p := newPublisher(t)
		p.NodeConfig.Spec.Network.ServiceCIDR = "invalid"
		assert.ErrorContains(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()), "failed to parse service CIDR")
	})

	t.Run("manifests directory is a file", func(t *testing.T) {
		p := newPublisher(t)
		require.NoError(t, os.MkdirAll(filepath.Dir(p.manifestDir), 0o700))
		require.NoError(t, os.WriteFile(p.manifestDir, nil, 0o600))
		assert.Error(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
	})

	t.Run("unwritable manifest", func(t *testing.T) {
		p := newPublisher(t)
		require.NoError(t, os.MkdirAll(filepath.Join(p.manifestDir, "cluster-info.yaml"), 0o700))
		assert.Error(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
	})

	t.Run("invalid API address", func(t *testing.T) {
		p := newPublisher(t)
		p.NodeConfig.Spec.API.Address = "["
		assert.ErrorContains(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()), "failed to determine the control plane endpoint")
	})
}

func TestPublisher_Lifecycle(t *testing.T) {
	p := newPublisher(t)
	manifest := filepath.Join(p.manifestDir, "cluster-info.yaml")

	require.NoError(t, p.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, p.Stop()) })
	require.NoError(t, p.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.FileExists(ct, manifest)
	}, 10*time.Second, 50*time.Millisecond)

	// The node-local configuration only changes with a restart.
	require.NoError(t, p.Stop())
	p.NodeConfig.Spec.API.ClusterInfoDiscovery = false
	require.NoError(t, p.Start(t.Context()))
	require.NoError(t, p.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.NoDirExists(ct, p.manifestDir)
	}, 10*time.Second, 50*time.Millisecond)
}

func TestPublisher_StopWithoutStart(t *testing.T) {
	assert.NoError(t, newPublisher(t).Stop())
}

func TestPublisher_LogsSyncErrors(t *testing.T) {
	p := newPublisher(t)
	require.NoError(t, os.Remove(filepath.Join(p.K0sVars.CertRootDir, "ca.crt")))
	logger, hook := logtest.NewNullLogger()
	p.log = logger

	require.NoError(t, p.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, p.Stop()) })
	require.NoError(t, p.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	// Failed syncs don't stop the publisher, they're retried.
	assert.Eventually(t, func() bool {
		for _, entry := range hook.AllEntries() {
			if entry.Message == "Failed to update the manifests, retrying" {
				return true
			}
		}
		return false
	}, 10*time.Second, 50*time.Millisecond)
}

func TestPublisher_WarnsOnLeaderAddress(t *testing.T) {
	apiServers := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault, Name: "kubernetes",
			Labels: map[string]string{discoveryv1.LabelServiceName: "kubernetes"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.1"}}, {Addresses: []string{"10.0.0.2"}}},
	}
	warnings := func(t *testing.T, externalAddress string) int {
		t.Helper()
		p := newPublisher(t, apiServers)
		p.NodeConfig.Spec.API.ExternalAddress = externalAddress
		logger, hook := logtest.NewNullLogger()
		p.log = logger
		for range 3 {
			require.NoError(t, p.sync(t.Context(), v1beta1.DefaultClusterConfig()))
		}
		var count int
		for _, entry := range hook.AllEntries() {
			if entry.Message != "" && entry.Level.String() == "warning" {
				count++
			}
		}
		return count
	}

	assert.Equal(t, 1, warnings(t, ""), "warned once for several controllers without a stable endpoint")
	assert.Zero(t, warnings(t, "lb.example.com"), "an external address is stable")
}

func TestKubernetesVersion(t *testing.T) {
	original := build.KubernetesVersion
	t.Cleanup(func() { build.KubernetesVersion = original })

	build.KubernetesVersion = "1.36.4"
	assert.Equal(t, "v1.36.4", kubernetesVersion())

	build.KubernetesVersion = ""
	assert.Equal(t, constant.KubeProxyImageVersion, kubernetesVersion())
}
