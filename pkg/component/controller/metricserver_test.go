/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
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
	ctx := context.Background()

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

	metrics := NewMetricServer(k0sVars, fakeFactory)
	require.NoError(t, metrics.Reconcile(ctx, cfg))
	metricsCfg, err := metrics.getConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, "100m", metricsCfg.CPURequest)
	require.Equal(t, "300M", metricsCfg.MEMRequest)
}
