// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

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

func TestCoreDNS_Reconcile_Leading(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := &CoreDNS{
		clusterDomain: "cluster.local",
		dnsAddress:    "10.96.0.10",
		client:        metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme()),
		clientFactory: clients,
		leaderStatus:  func() (leaderelection.Status, <-chan struct{}) { return leaderelection.StatusLeading, nil },
		log:           logrus.WithField("component", "coredns"),
	}

	require.NoError(t, c.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	_, err := clients.Client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
	assert.NoError(t, err, "expected the coredns ConfigMap to be applied")
}

func TestCoreDNS_Reconcile_NotLeading(t *testing.T) {
	clients := testutil.NewFakeClientFactory()
	c := &CoreDNS{
		clusterDomain: "cluster.local",
		dnsAddress:    "10.96.0.10",
		client:        metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme()),
		clientFactory: clients,
		leaderStatus:  func() (leaderelection.Status, <-chan struct{}) { return leaderelection.StatusPending, nil },
		log:           logrus.WithField("component", "coredns"),
	}

	require.NoError(t, c.Reconcile(t.Context(), v1beta1.DefaultClusterConfig()))

	_, err := clients.Client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "a non-leader must not apply the coredns stack, got: %v", err)
	lastKnownClusterConfig, _ := c.lastKnownClusterConfig.Peek()
	assert.NotNil(t, lastKnownClusterConfig, "lastKnownClusterConfig must still be tracked so a later leadership change gets picked up")
}

// Test that the reconcile handles ordering and config snapshotting properly
// in case concurrent triggering
func TestCoreDNS_Reconcile_UsesLatestConfigAfterConcurrentUpdate(t *testing.T) {
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

	c := &CoreDNS{
		clusterDomain: "cluster.local",
		dnsAddress:    "10.96.0.10",
		client:        metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme()),
		clientFactory: clients,
		leaderStatus:  func() (leaderelection.Status, <-chan struct{}) { return leaderelection.StatusLeading, nil },
		log:           logrus.WithField("component", "coredns"),
	}

	configA := v1beta1.DefaultClusterConfig()
	configA.Spec.Images.CoreDNS.Image = "coredns-a.example/test"
	configB := configA.DeepCopy()
	configB.Spec.Images.CoreDNS.Image = "coredns-b.example/test"

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- c.Reconcile(t.Context(), configA)
	}()

	<-applyStarted

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- c.Reconcile(t.Context(), configB)
	}()

	require.Eventually(t, func() bool {
		latest, _ := c.lastKnownClusterConfig.Peek()
		return latest == configB
	}, time.Second, time.Millisecond, "new config was not published")

	close(releaseApply)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)

	deployment, err := clients.Client.AppsV1().Deployments(metav1.NamespaceSystem).Get(t.Context(), "coredns", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, configB.Spec.Images.CoreDNS.URI(), deployment.Spec.Template.Spec.Containers[0].Image, "latest config should win after concurrent reconciliation")
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
