// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

// linuxNodes builds the Linux nodes whose count drives the CoreDNS replica
// count. Seed them into the client factory, so that they're visible through its
// metadata client.
func linuxNodes(count int) []runtime.Object {
	nodes := make([]runtime.Object, 0, count)
	for i := range count {
		nodes = append(nodes, &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   fmt.Sprintf("node-%d", i),
				Labels: map[string]string{corev1.LabelOSStable: string(corev1.Linux)},
			},
		})
	}
	return nodes
}

// windowsNode is a node that must not be counted towards the replica count.
func windowsNode() runtime.Object {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "windows-node",
			Labels: map[string]string{corev1.LabelOSStable: string(corev1.Windows)},
		},
	}
}

func newTestCoreDNS(t *testing.T, clients *testutil.FakeClientFactory, status leaderelection.Status) *CoreDNS {
	t.Helper()

	return &CoreDNS{
		clusterDomain: "cluster.local",
		dnsAddress:    "10.96.0.10",
		client:        clients.MetadataClient,
		clientFactory: clients,
		leaderStatus:  func() (leaderelection.Status, <-chan struct{}) { return status, nil },
		log:           logrus.WithField("component", "coredns"),
	}
}

func getCoreDNSDeployment(t *testing.T, clients *testutil.FakeClientFactory) (*appsv1.Deployment, error) {
	t.Helper()
	return clients.Client.AppsV1().Deployments(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
}

func TestCoreDNS_RenderWithPatch(t *testing.T) {
	cfg := coreDNSConfig{
		Replicas:      1,
		ClusterDomain: "cluster.local",
		ClusterDNSIP:  "10.96.0.10",
		Image:         "coredns:latest",
		PullPolicy:    "IfNotPresent",
	}
	tw := templatewriter.TemplateWriter{
		Name:     "coredns",
		Template: coreDNSTemplate,
		Data:     cfg,
		Patches: v1beta1.Patches{{
			Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"metadata":{"annotations":{"patched":"true"}}}`},
		}},
	}
	var buf bytes.Buffer
	require.NoError(t, tw.WriteToBuffer(&buf))
	assert.Contains(t, buf.String(), "patched")
}

// The tests below drive reconcile directly. It is synchronous and takes the
// cluster config as an argument, so there's no need to involve Start and its
// goroutine in order to cover the actual reconciliation behavior

func TestCoreDNS_reconcile_Leading(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

	require.NoError(t, c.reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	_, err := clients.Client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
	assert.NoError(t, err, "expected the coredns ConfigMap to be applied")
	_, err = getCoreDNSDeployment(t, clients)
	assert.NoError(t, err, "expected the coredns Deployment to be applied")
}

func TestCoreDNS_reconcile_NotLeading(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusPending)

	require.NoError(t, c.reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	_, err := clients.Client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "a non-leader must not apply the coredns stack, got: %v", err)
}

func TestCoreDNS_reconcile_AppliesPatches(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

	clusterConfig := v1beta1.DefaultClusterConfig()
	clusterConfig.Spec.Network.CoreDNS = &v1beta1.CoreDNS{
		Patches: v1beta1.Patches{{
			Target: v1beta1.PatchTarget{Kind: "Deployment", Name: "coredns"},
			Patch:  v1beta1.PatchSpec{Type: v1beta1.MergePatchType, Content: `{"metadata":{"annotations":{"patched":"true"}}}`},
		}},
	}

	require.NoError(t, c.reconcile(t.Context(), clusterConfig))

	deployment, err := getCoreDNSDeployment(t, clients)
	require.NoError(t, err)
	assert.Equal(t, "true", deployment.Annotations["patched"], "the patch must reach the applied resource, not just the rendered template")
}

func TestCoreDNS_reconcile_ScalesWithNodeCount(t *testing.T) {
	for _, test := range []struct {
		name                   string
		nodes                  int
		replicas               int32
		maxUnavailable         string
		expectPodAntiAffinity  bool
		expectDisruptionBudget bool
	}{
		{"a single node gets neither anti-affinity nor a PDB", 1, 1, "", false, false},
		{"two nodes get anti-affinity, a PDB and maxUnavailable=1", 2, 2, "1", true, true},
		{"fifteen nodes scale to three replicas, still maxUnavailable=1", 15, 3, "1", true, true},
		{"thirty nodes scale to four replicas, back to the Kubernetes default", 30, 4, "", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The Windows node is in there to ensure it's filtered out and
			// doesn't influence the replica count in any way.
			clients := testutil.NewFakeClientFactory(append(linuxNodes(test.nodes), windowsNode())...)
			c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

			require.NoError(t, c.reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

			deployment, err := getCoreDNSDeployment(t, clients)
			require.NoError(t, err)
			if assert.NotNil(t, deployment.Spec.Replicas) {
				assert.Equal(t, test.replicas, *deployment.Spec.Replicas)
			}

			if rollingUpdate := deployment.Spec.Strategy.RollingUpdate; test.maxUnavailable == "" {
				assert.Nil(t, rollingUpdate, "expected the Kubernetes default rolling update strategy")
			} else if assert.NotNil(t, rollingUpdate) {
				assert.Equal(t, test.maxUnavailable, rollingUpdate.MaxUnavailable.String())
			}

			podSpec := deployment.Spec.Template.Spec
			assert.Equal(t, test.expectPodAntiAffinity, podSpec.Affinity != nil && podSpec.Affinity.PodAntiAffinity != nil)

			_, err = clients.Client.PolicyV1().PodDisruptionBudgets(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
			if test.expectDisruptionBudget {
				assert.NoError(t, err, "expected a PodDisruptionBudget")
			} else {
				assert.True(t, apierrors.IsNotFound(err), "expected no PodDisruptionBudget, got: %v", err)
			}
		})
	}
}

func TestCoreDNS_reconcile_SkipsRedundantApply(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)
	clusterConfig := v1beta1.DefaultClusterConfig()

	require.NoError(t, c.reconcile(t.Context(), clusterConfig))
	require.NotEmpty(t, clients.DynamicClient.Actions(), "the first reconciliation has to talk to the API server")

	clients.DynamicClient.ClearActions()
	require.NoError(t, c.reconcile(t.Context(), clusterConfig))
	assert.Empty(t, clients.DynamicClient.Actions(), "an unchanged configuration must not be applied again")
}

// The remaining tests cover the wiring between the Reconcile entry point and
// the goroutine started by Start, i.e. that a published cluster config actually
// ends up being applied.

func TestCoreDNS_Reconcile_PublishesWithoutApplying(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

	// Reconcile is fire-and-forget: it only publishes the config. Without Start
	// there's nobody around to pick it up.
	require.NoError(t, c.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	lastKnownClusterConfig, _ := c.lastKnownClusterConfig.Peek()
	assert.NotNil(t, lastKnownClusterConfig, "lastKnownClusterConfig must be tracked so that a later leadership change picks it up")
	assert.Empty(t, clients.DynamicClient.Actions(), "Reconcile alone must not apply anything")
}

func TestCoreDNS_Reconcile_AppliesLatestConfig(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

	require.NoError(t, c.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })

	configA := v1beta1.DefaultClusterConfig()
	configA.Spec.Images.CoreDNS.Image = "coredns-a.example/test"
	configB := configA.DeepCopy()
	configB.Spec.Images.CoreDNS.Image = "coredns-b.example/test"

	require.NoError(t, c.Reconcile(t.Context(), configA))
	assertCoreDNSImage(t, clients, configA)

	require.NoError(t, c.Reconcile(t.Context(), configB))
	assertCoreDNSImage(t, clients, configB)
}

// A config published while an apply is in flight must not be lost: the loop is
// still holding the expiration channel that got closed during that apply, so
// the next iteration has to pick up the newer config.
func TestCoreDNS_Reconcile_ConfigChangeDuringApplyIsNotLost(t *testing.T) {
	clients := testutil.NewFakeClientFactory()

	applyStarted := make(chan struct{})
	releaseApply := make(chan struct{})
	var blockOnce sync.Once
	clients.DynamicClient.PrependReactor("create", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		blockOnce.Do(func() {
			close(applyStarted)
			<-releaseApply
		})
		return false, nil, nil
	})

	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)
	require.NoError(t, c.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })

	configA := v1beta1.DefaultClusterConfig()
	configA.Spec.Images.CoreDNS.Image = "coredns-a.example/test"
	configB := configA.DeepCopy()
	configB.Spec.Images.CoreDNS.Image = "coredns-b.example/test"

	require.NoError(t, c.Reconcile(t.Context(), configA))
	select {
	case <-applyStarted:
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the first apply never started")
	}

	// Publish the new config while the first apply is blocked, then let it finish.
	require.NoError(t, c.Reconcile(t.Context(), configB))
	close(releaseApply)

	assertCoreDNSImage(t, clients, configB)
}

// Only the leader applies the stack, so anything this controller applied may
// have been changed by another one while it wasn't leading. Regaining the lead
// therefore has to re-apply, even though the cluster config didn't change.
func TestCoreDNS_Reconcile_ReappliesAfterRegainingLead(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := newTestCoreDNS(t, clients, leaderelection.StatusLeading)

	// A Latest of leader election statuses is exactly a leaderelection.StatusFunc.
	leaderStatus := value.NewLatest(leaderelection.StatusLeading)
	c.leaderStatus = leaderStatus.Peek

	require.NoError(t, c.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })

	clusterConfig := v1beta1.DefaultClusterConfig()
	require.NoError(t, c.Reconcile(t.Context(), clusterConfig))
	assertCoreDNSImage(t, clients, clusterConfig)

	// Simulate the cluster drifting away from what this controller applied.
	require.NoError(t, clients.Client.AppsV1().Deployments(metav1.NamespaceSystem).
		Delete(t.Context(), "coredns", metav1.DeleteOptions{}))

	// Hand the lead away and take it back. The cluster config is unchanged, so
	// only dropping the record of the previous apply can bring the Deployment
	// back before the periodic reconciliation would.
	leaderStatus.Set(leaderelection.StatusPending)
	leaderStatus.Set(leaderelection.StatusLeading)

	assertCoreDNSImage(t, clients, clusterConfig)
}

func assertCoreDNSImage(t *testing.T, clients *testutil.FakeClientFactory, clusterConfig *v1beta1.ClusterConfig) {
	t.Helper()

	ctx := t.Context()
	expected := clusterConfig.Spec.Images.CoreDNS.URI()

	// Keep the timeout well below the reconciler's ticker interval. Otherwise a
	// config change that failed to wake the loop would still be picked up by the
	// periodic reconciliation, and these tests couldn't tell the two apart.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		deployment, err := clients.Client.AppsV1().Deployments(metav1.NamespaceSystem).Get(ctx, "coredns", metav1.GetOptions{})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, expected, deployment.Spec.Template.Spec.Containers[0].Image)
	}, 5*time.Second, 10*time.Millisecond, "the coredns Deployment never converged on %s", expected)
}

func Test_replicaCount(t *testing.T) {
	tests := []struct {
		name  string
		nodes int
		want  int
	}{
		{
			"one replica even for zero nodes",
			0,
			1,
		},
		{
			"one replica for one node",
			1,
			1,
		},
		{
			"2 replicas for two nodes (1 + ceil(2/10)) ",
			2,
			2,
		},
		{
			"2 replicas for 10 nodes (1 + 10/10)",
			10,
			2,
		},
		{
			"3 replicas for 15 nodes (1 + ceil(15/10))",
			15,
			3,
		},
		{
			"3 replicas for 20 nodes (1 + (20/10))",
			20,
			3,
		},
		{
			"11 replicas for 100 nodes (1 + (100/10) )",
			100,
			11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicaCount(tt.nodes); got != tt.want {
				t.Errorf("replicaCount() = %v, want %v", got, tt.want)
			}
		})
	}
}
