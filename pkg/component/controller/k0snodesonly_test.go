// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"slices"
	"testing"
	"testing/synctest"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requiresK0sNode reports whether the pod spec keeps off nodes without k0s.
func requiresK0sNode(pod *corev1.PodSpec) bool {
	if pod.Affinity == nil || pod.Affinity.NodeAffinity == nil || pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return false
	}
	for _, term := range pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == constant.K0sNodeLabel && expr.Operator == corev1.NodeSelectorOpNotIn && slices.Equal(expr.Values, []string{"false"}) {
				return true
			}
		}
	}
	return false
}

func nodeLocalLoadBalancingConfig(nllb bool) *v1beta1.ClusterConfig {
	cfg := v1beta1.DefaultClusterConfig()
	cfg.Spec.Network.NodeLocalLoadBalancing.Enabled = nllb
	return cfg
}

func TestKubeProxy_K0sNodesOnly(t *testing.T) {
	for _, test := range []struct {
		name string
		nllb bool
	}{
		{"default", false},
		{"nllb", true},
	} {
		synctest.Test(t, func(t *testing.T) {
			_, manifestsDir := startComponent(t, nodeLocalLoadBalancingConfig(test.nllb))
			_, daemonSet, _ := awaitUpdate(t, manifestsDir, nil)
			// Only nodes with k0s run the node-local load balancer.
			assert.Equal(t, test.nllb, requiresK0sNode(&daemonSet.Spec.Template.Spec), test.name)
		})
	}
}

func TestKonnectivityAgent_K0sNodesOnly(t *testing.T) {
	for _, test := range []struct {
		name string
		nllb bool
	}{
		{"default", false},
		{"nllb", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifestsDir := t.TempDir()
			underTest := KonnectivityAgent{
				ManifestsDir:           manifestsDir,
				KonnectivityServerHost: "10.0.0.1",
				EventEmitter:           prober.NewEventEmitter(),
			}
			require.NoError(t, underTest.Init(t.Context()))
			require.NoError(t, underTest.writeKonnectivityAgent(nodeLocalLoadBalancingConfig(test.nllb), 1))
			daemonSet := loadDaemonSet(t, manifestsDir)
			assert.Equal(t, test.nllb, requiresK0sNode(&daemonSet.Spec.Template.Spec))
		})
	}
}
