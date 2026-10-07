// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package externalnodes

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newComponent(t *testing.T, objects ...runtime.Object) *Component {
	t.Helper()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	nodeConfig := v1beta1.DefaultClusterConfig()
	nodeConfig.Spec.API.Address = "10.0.0.1"
	c := &Component{K0sVars: k0sVars, NodeConfig: nodeConfig, Clients: testutil.NewFakeClientFactory(objects...)}
	require.NoError(t, c.Init(t.Context()))
	return c
}

func clusterConfig(nllb bool) *v1beta1.ClusterConfig {
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.Network.NodeLocalLoadBalancing.Enabled = nllb
	return cfg
}

// externalNode returns a node that k0s labeled as not running k0s.
func externalNode() *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{constant.K0sNodeLabel: "false"}}}
}

// fakeClient returns the fake clientset behind the component's client factory.
func fakeClient(t *testing.T, c *Component) *fake.Clientset {
	t.Helper()
	client, err := c.Clients.GetClient()
	require.NoError(t, err)
	clientset, ok := client.(*fake.Clientset)
	require.True(t, ok)
	return clientset
}

// failGet lets the fake API server fail to get the named object of the resource.
func failGet(client *fake.Clientset, resource, name string) {
	client.PrependReactor("get", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		get, ok := action.(k8stesting.GetAction)
		if !ok || get.GetName() != name {
			return false, nil, nil
		}
		return true, nil, errors.New("injected error")
	})
}

// failingClients is a client factory that can't create clients.
type failingClients struct {
	kubeutil.ClientFactoryInterface
}

func (failingClients) GetClient() (kubernetes.Interface, error) {
	return nil, errors.New("no client")
}

func loadManifest(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	resources, err := applier.ReadUnstructuredStream(bytes.NewReader(data), path)
	require.NoError(t, err)
	names := map[string]string{}
	for _, r := range resources {
		names[r.GetKind()+"/"+r.GetName()] = r.GetNamespace()
	}
	return names
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

func TestComponent_Sync(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	node := externalNode()
	c := newComponent(t, cm, ds, konnectivitySource(), node)
	manifest := filepath.Join(c.manifestDir, "daemonsets.yaml")

	require.NoError(t, c.sync(t.Context(), clusterConfig(false)))
	assert.NoDirExists(t, c.manifestDir, "no copies without node-local load balancing")

	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))
	copies := loadManifest(t, manifest)
	assert.Contains(t, copies, "DaemonSet/kube-proxy-external")
	assert.Contains(t, copies, "ConfigMap/kube-proxy-external")
	assert.Contains(t, copies, "DaemonSet/konnectivity-agent-external")

	require.NoError(t, c.sync(t.Context(), clusterConfig(false)))
	assert.NoDirExists(t, c.manifestDir, "copies are removed along with node-local load balancing")

	// Once the nodes without k0s are gone, the copies go as well.
	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))
	assert.FileExists(t, manifest)
	node.Labels = map[string]string{constant.K0sNodeLabel: "true"}
	_, err := fakeClient(t, c).CoreV1().Nodes().Update(t.Context(), node, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))
	assert.NoDirExists(t, c.manifestDir)
}

func TestComponent_NoStackWithoutExternalNodes(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	c := newComponent(t, cm, ds, konnectivitySource())
	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))
	assert.NoDirExists(t, c.manifestDir, "clusters without such nodes get no stack")
}

func TestComponent_NoCopiesWithoutLocalhost(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://10.0.0.1:6443")
	konnectivity := konnectivitySource()
	konnectivity.Spec.Template.Spec.Containers[0].Args[1] = "--proxy-server-host=konnectivity.example.com"
	c := newComponent(t, cm, ds, konnectivity, externalNode())

	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))
	assert.NoDirExists(t, c.manifestDir)
}

func TestComponent_SyncWithControlPlaneLoadBalancing(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	c := newComponent(t, cm, ds, konnectivitySource(), externalNode())
	c.NodeConfig.Spec.Network.ControlPlaneLoadBalancing = &v1beta1.ControlPlaneLoadBalancingSpec{
		Enabled: true, Type: v1beta1.CPLBTypeKeepalived,
		Keepalived: &v1beta1.KeepalivedSpec{VRRPInstances: v1beta1.VRRPInstances{{VirtualIPs: []string{"10.0.0.100/24"}}}},
	}
	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))

	// The copies use the virtual IP, which moves on failover.
	manifest := filepath.Join(c.manifestDir, "daemonsets.yaml")
	kubeProxy := manifestObject(t, manifest, "ConfigMap", "kube-proxy-external")
	kubeconfig, _, err := unstructured.NestedString(kubeProxy.Object, "data", kubeProxyConfKey)
	require.NoError(t, err)
	assert.Contains(t, kubeconfig, "server: https://10.0.0.100:6443")

	konnectivity := manifestObject(t, manifest, "DaemonSet", "konnectivity-agent-external")
	containers, _, err := unstructured.NestedSlice(konnectivity.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.Len(t, containers, 1)
	container, ok := containers[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, container["args"], "--proxy-server-host=10.0.0.100")
}

func TestComponent_KubeProxyWithoutDaemonSet(t *testing.T) {
	cm, _ := kubeProxySources(t, "https://localhost:7443")
	c := newComponent(t, cm, konnectivitySource(), externalNode())
	require.NoError(t, c.sync(t.Context(), clusterConfig(true)))

	copies := loadManifest(t, filepath.Join(c.manifestDir, "daemonsets.yaml"))
	assert.Equal(t, map[string]string{"DaemonSet/konnectivity-agent-external": "kube-system"}, copies,
		"kube-proxy is only copied along with its DaemonSet")
}

func TestComponent_KonnectivityAgentPort(t *testing.T) {
	c := newComponent(t, konnectivitySource(), externalNode())
	cfg := clusterConfig(true)
	cfg.Spec.Konnectivity.AgentPort = 9132
	require.NoError(t, c.sync(t.Context(), cfg))

	konnectivity := manifestObject(t, filepath.Join(c.manifestDir, "daemonsets.yaml"), "DaemonSet", "konnectivity-agent-external")
	containers, _, err := unstructured.NestedSlice(konnectivity.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	require.Len(t, containers, 1)
	container, ok := containers[0].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, container["args"], "--proxy-server-port=9132")
}

func TestComponent_SyncErrors(t *testing.T) {
	t.Run("invalid API address", func(t *testing.T) {
		c := newComponent(t, externalNode())
		c.NodeConfig.Spec.API.Address = "["
		assert.ErrorContains(t, c.sync(t.Context(), clusterConfig(true)), "failed to determine the control plane endpoint")
	})

	t.Run("no client", func(t *testing.T) {
		c := newComponent(t)
		c.Clients = failingClients{}
		assert.ErrorContains(t, c.sync(t.Context(), clusterConfig(true)), "no client")
	})

	t.Run("manifests directory is a file", func(t *testing.T) {
		cm, ds := kubeProxySources(t, "https://localhost:7443")
		c := newComponent(t, cm, ds, externalNode())
		require.NoError(t, os.MkdirAll(filepath.Dir(c.manifestDir), 0o700))
		require.NoError(t, os.WriteFile(c.manifestDir, nil, 0o600))
		assert.Error(t, c.sync(t.Context(), clusterConfig(true)))
	})

	for _, test := range []struct{ resource, name string }{
		{"configmaps", kubeProxyName},
		{"daemonsets", kubeProxyName},
		{"daemonsets", konnectivityName},
	} {
		t.Run("failing get of "+test.resource+"/"+test.name, func(t *testing.T) {
			cm, ds := kubeProxySources(t, "https://localhost:7443")
			c := newComponent(t, cm, ds, konnectivitySource(), externalNode())
			failGet(fakeClient(t, c), test.resource, test.name)
			assert.ErrorContains(t, c.sync(t.Context(), clusterConfig(true)), "injected error")
		})
	}
}

func TestComponent_FollowsNodes(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	c := newComponent(t, cm, ds, konnectivitySource())
	// Only changes of nodes trigger syncs in time.
	c.resync = time.Hour
	client := fakeClient(t, c)

	require.NoError(t, c.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })
	require.NoError(t, c.Reconcile(t.Context(), clusterConfig(true)))
	assert.Never(t, func() bool { _, err := os.Stat(c.manifestDir); return err == nil }, 500*time.Millisecond, 50*time.Millisecond)

	_, err := client.CoreV1().Nodes().Create(t.Context(), externalNode(), metav1.CreateOptions{})
	require.NoError(t, err)
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.FileExists(ct, filepath.Join(c.manifestDir, "daemonsets.yaml"))
	}, 10*time.Second, 50*time.Millisecond)

	require.NoError(t, client.CoreV1().Nodes().Delete(t.Context(), "other", metav1.DeleteOptions{}))
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.NoDirExists(ct, c.manifestDir)
	}, 10*time.Second, 50*time.Millisecond)
}

func TestComponent_StopWithoutStart(t *testing.T) {
	assert.NoError(t, newComponent(t).Stop())
}

func TestComponent_LogsSyncErrors(t *testing.T) {
	c := newComponent(t)
	c.Clients = failingClients{}
	logger, hook := logtest.NewNullLogger()
	c.log = logger

	require.NoError(t, c.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })
	require.NoError(t, c.Reconcile(t.Context(), clusterConfig(true)))

	// Failed syncs don't stop the component, they're retried.
	assert.Eventually(t, func() bool {
		for _, entry := range hook.AllEntries() {
			if entry.Message == "Failed to update the manifests, retrying" {
				return true
			}
		}
		return false
	}, 10*time.Second, 50*time.Millisecond)
}

func TestUsesLocalhost(t *testing.T) {
	cm, _ := kubeProxySources(t, "https://localhost:7443")
	assert.True(t, usesLocalhost(cm))

	cm, _ = kubeProxySources(t, "https://10.0.0.1:6443")
	assert.False(t, usesLocalhost(cm))

	cm.Data[kubeProxyConfKey] = "invalid"
	assert.False(t, usesLocalhost(cm))
}

func TestHasArg(t *testing.T) {
	ds := konnectivitySource()
	assert.True(t, hasArg(ds, "--proxy-server-host=localhost"))
	assert.False(t, hasArg(ds, "--proxy-server-host=10.0.0.1"))
}
