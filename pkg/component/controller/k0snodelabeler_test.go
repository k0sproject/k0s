// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/constant"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLeader is a leader elector whose status the test controls.
type testLeader struct {
	status value.Latest[leaderelection.Status]
}

func (l *testLeader) CurrentStatus() (leaderelection.Status, <-chan struct{}) { return l.status.Peek() }
func (l *testLeader) IsLeader() bool {
	s, _ := l.status.Peek()
	return s == leaderelection.StatusLeading
}
func (*testLeader) AddAcquiredLeaseCallback(func()) {}
func (*testLeader) AddLostLeaseCallback(func())     {}

// failingClients is a client factory that can't create clients.
type failingClients struct {
	kubeutil.ClientFactoryInterface
}

func (failingClients) GetClient() (kubernetes.Interface, error) {
	return nil, errors.New("no client")
}

func labeledNode(name, version string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: version}},
	}
}

func newK0sNodeLabeler(t *testing.T, objects ...runtime.Object) (*K0sNodeLabeler, *fake.Clientset) {
	t.Helper()
	l := &K0sNodeLabeler{Clients: testutil.NewFakeClientFactory(objects...), LeaderElector: leaderelector.Off(), resync: 100 * time.Millisecond}
	require.NoError(t, l.Init(t.Context()))
	client, err := l.Clients.GetClient()
	require.NoError(t, err)
	clientset, ok := client.(*fake.Clientset)
	require.True(t, ok)
	return l, clientset
}

// k0sLabel returns the k0s label of the node, if it can be read and is set.
func k0sLabel(t *testing.T, client kubernetes.Interface, name string) (string, bool) {
	t.Helper()
	node, err := client.CoreV1().Nodes().Get(t.Context(), name, metav1.GetOptions{})
	if err != nil {
		return "", false
	}
	value, ok := node.Labels[constant.K0sNodeLabel]
	return value, ok
}

func TestK0sNodeLabeler_LabelNodes(t *testing.T) {
	l, client := newK0sNodeLabeler(t,
		labeledNode("other", "v1.36.4", nil),
		labeledNode("k0s", "v1.36.4+k0s", map[string]string{constant.K0sNodeLabel: "true"}),
		labeledNode("upgraded", "v1.36.4+k0s", nil),
		labeledNode("controller", "v1.36.4", map[string]string{constant.K0SNodeRoleLabel: "control-plane"}),
	)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); l.labelNodes(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	label := func(name string) string { value, _ := k0sLabel(t, client, name); return value }
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, "false", label("other"))
		assert.Equal(t, "true", label("k0s"))
		assert.Equal(t, "true", label("upgraded"), "k0s workers that registered without the label")
		assert.Equal(t, "true", label("controller"), "controllers that run workloads")
	}, 10*time.Second, 50*time.Millisecond)

	// Wrong values on k0s nodes are corrected.
	_, err := client.CoreV1().Nodes().Update(t.Context(), labeledNode("k0s", "v1.36.4+k0s", map[string]string{constant.K0sNodeLabel: "false"}), metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, "true", label("k0s"))
	}, 10*time.Second, 50*time.Millisecond)
}

func TestK0sNodeLabeler_LabelsNodesWhileLeading(t *testing.T) {
	l, client := newK0sNodeLabeler(t, labeledNode("other", "v1.36.4", nil))
	leader := &testLeader{}
	leader.status.Set(leaderelection.StatusPending)
	l.LeaderElector = leader
	labeled := func() bool { _, ok := k0sLabel(t, client, "other"); return ok }

	require.NoError(t, l.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, l.Stop()) })

	assert.Never(t, labeled, 500*time.Millisecond, 50*time.Millisecond, "only the leader labels nodes")
	leader.status.Set(leaderelection.StatusLeading)
	assert.Eventually(t, labeled, 10*time.Second, 50*time.Millisecond)
}

func TestK0sNodeLabeler_StopWithoutStart(t *testing.T) {
	l, _ := newK0sNodeLabeler(t)
	assert.NoError(t, l.Stop())
}

func TestK0sNodeLabeler_LabelNodeErrors(t *testing.T) {
	l, client := newK0sNodeLabeler(t)

	// Objects other than nodes are ignored.
	l.labelNode(t.Context(), client, "not a node")
	assert.Empty(t, client.Actions())

	// Failed patches are logged and retried on the next resync.
	client.PrependReactor("patch", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("injected error")
	})
	l.labelNode(t.Context(), client, labeledNode("other", "v1.36.4", nil))
	require.Len(t, client.Actions(), 1)
	assert.Equal(t, "patch", client.Actions()[0].GetVerb())
}

func TestK0sNodeLabeler_LabelNodesWithoutClient(t *testing.T) {
	l, _ := newK0sNodeLabeler(t)
	l.Clients = failingClients{}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	// It keeps retrying until the context is done.
	l.labelNodes(ctx)
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestK0sNodeLabelPatch(t *testing.T) {
	const (
		k0s   = "v1.36.4+k0s"
		other = "v1.36.4"
	)
	for _, test := range []struct {
		labels  map[string]string
		name    string
		version string
		want    string
	}{
		{nil, "unreported version", "", ""},
		{nil, "unlabeled other node", other, `{"metadata":{"labels":{"node.k0sproject.io/k0s":"false"}}}`},
		{map[string]string{constant.K0sNodeLabel: "false"}, "other node", other, ""},
		{map[string]string{constant.K0sNodeLabel: "true"}, "k0s worker with an upstream kubelet", other, ""},
		{map[string]string{constant.K0SNodeRoleLabel: "control-plane"}, "controller with an upstream kubelet", other, `{"metadata":{"labels":{"node.k0sproject.io/k0s":"true"}}}`},
		{map[string]string{constant.K0sNodeLabel: "maybe"}, "unknown value on other node", other, `{"metadata":{"labels":{"node.k0sproject.io/k0s":"false"}}}`},
		{nil, "unlabeled k0s worker", k0s, `{"metadata":{"labels":{"node.k0sproject.io/k0s":"true"}}}`},
		{map[string]string{constant.K0sNodeLabel: "true"}, "k0s worker", k0s, ""},
		{map[string]string{constant.K0sNodeLabel: "false"}, "wrong value", k0s, `{"metadata":{"labels":{"node.k0sproject.io/k0s":"true"}}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			patch, err := k0sNodeLabelPatch(labeledNode("", test.version, test.labels))
			require.NoError(t, err)
			assert.Equal(t, test.want, string(patch))
		})
	}
}

func FuzzK0sNodeLabelPatch(f *testing.F) {
	f.Add("v1.36.4+k0s", "true", true, false)
	f.Add("v1.36.4", "false", true, true)
	f.Add("", "x", false, false)
	f.Fuzz(func(t *testing.T, version, k0sValue string, k0sLabeled, controller bool) {
		node := labeledNode("", version, map[string]string{})
		if k0sLabeled {
			node.Labels[constant.K0sNodeLabel] = k0sValue
		}
		if controller {
			node.Labels[constant.K0SNodeRoleLabel] = "control-plane"
		}

		patch, err := k0sNodeLabelPatch(node)
		require.NoError(t, err)
		if patch == nil {
			return
		}
		var merge struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		}
		require.NoError(t, json.Unmarshal(patch, &merge))
		maps.Copy(node.Labels, merge.Metadata.Labels)

		// Labeling converges on either value.
		patch, err = k0sNodeLabelPatch(node)
		require.NoError(t, err)
		assert.Nil(t, patch, "%s", patch)
		assert.Contains(t, []string{"true", "false"}, node.Labels[constant.K0sNodeLabel])
	})
}
