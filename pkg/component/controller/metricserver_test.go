package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var cfg = v1beta1.DefaultClusterConfig()

func TestGetConfigWithZeroNodes(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	ctx := context.Background()

	metrics, err := NewMetricServer(k0sVars, fakeFactory)
	require.NoError(t, err)
	require.NoError(t, metrics.Reconcile(ctx, cfg))
	cfg, err := metrics.getConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "10m", cfg.CPURequest)
	require.Equal(t, "30M", cfg.MEMRequest)
}

func TestGetConfigWithSomeNodes(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()
	fakeClient, _ := fakeFactory.GetClient()
	ctx := context.Background()

	for i := 1; i <= 100; i++ {
		n := &corev1.Node{
			ObjectMeta: v1.ObjectMeta{
				Name: fmt.Sprintf("node-%d", i),
			},
		}
		_, err := fakeClient.CoreV1().Nodes().Create(context.TODO(), n, v1.CreateOptions{})
		require.NoError(t, err)
	}

	metrics, err := NewMetricServer(k0sVars, fakeFactory)
	require.NoError(t, err)
	require.NoError(t, metrics.Reconcile(ctx, cfg))
	cfg, err := metrics.getConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "100m", cfg.CPURequest)
	require.Equal(t, "300M", cfg.MEMRequest)
}
