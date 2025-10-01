// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetConfigWithZeroNodes(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	fakeFactory := testutil.NewFakeClientFactory()
	ctx := t.Context()

	metrics := NewMetricServer(k0sVars, fakeFactory)
	require.NoError(t, metrics.Reconcile(ctx, cfg))
	metricsCfg, err := metrics.getConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "10m", metricsCfg.CPURequest)
	require.Equal(t, "30M", metricsCfg.MEMRequest)
}

func TestGetConfigWithSomeNodes(t *testing.T) {
	cfg := v1beta1.DefaultClusterConfig()
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	fakeFactory := testutil.NewFakeClientFactory()
	fakeClient, _ := fakeFactory.GetClient()
	ctx := t.Context()

	for i := 1; i <= 100; i++ {
		n := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: fmt.Sprintf("node-%d", i),
			},
		}
		_, err := fakeClient.CoreV1().Nodes().Create(t.Context(), n, v1.CreateOptions{})
		require.NoError(t, err)
	}

	metrics := NewMetricServer(k0sVars, fakeFactory)
	require.NoError(t, metrics.Reconcile(ctx, cfg))
	metricsCfg, err := metrics.getConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "100m", metricsCfg.CPURequest)
	require.Equal(t, "300M", metricsCfg.MEMRequest)
}
