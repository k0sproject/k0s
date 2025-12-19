// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"testing/synctest"

	internallog "github.com/k0sproject/k0s/internal/pkg/log"
	"github.com/k0sproject/k0s/internal/testutil"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/constant"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterConfigInitializer_Create(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clients := testutil.NewFakeClientFactory()
		leaderElector := leaderelector.Dummy{Leader: true}
		initialConfig := k0sv1beta1.DefaultClusterConfig()
		initialConfig.ResourceVersion = "42"

		underTest := controller.NewClusterConfigInitializer(
			clients, &leaderElector, initialConfig.DeepCopy(),
		)

		require.NoError(t, underTest.Init(t.Context()))
		require.NoError(t, underTest.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

		synctest.Wait()

		crds, err := clients.APIExtensionsClient.ApiextensionsV1().
			CustomResourceDefinitions().
			List(t.Context(), metav1.ListOptions{})
		if assert.NoError(t, err) && assert.Len(t, crds.Items, 1) {
			crd := crds.Items[0]
			assert.Equal(t, "clusterconfigs.k0s.k0sproject.io", crd.Name)
			assert.Equal(t, "api-config", crd.Labels["k0s.k0sproject.io/stack"])
		}
		actualConfig, err := clients.K0sClient.K0sV1beta1().
			ClusterConfigs(constant.ClusterConfigNamespace).
			Get(t.Context(), "k0s", metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.Equal(t, initialConfig, actualConfig)
		}
	})
}

func TestClusterConfigInitializer_NoConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clients := testutil.NewFakeClientFactory()
		leaderElector := leaderelector.Dummy{Leader: false}
		initialConfig := k0sv1beta1.DefaultClusterConfig()

		underTest := controller.NewClusterConfigInitializer(
			clients, &leaderElector, initialConfig.DeepCopy(),
		)

		ctx, cancel := context.WithCancelCause(t.Context())

		var gets uint
		abortTest := errors.New("aborting test after some retries")
		clients.K0sClient.PrependReactor("get", "clusterconfigs", func(action k8stesting.Action) (bool, runtime.Object, error) {
			gets++ // Let's observe some retries, then cancel the context.
			if gets > 1 {
				cancel(abortTest)
			}
			return false, nil, nil
		})

		require.NoError(t, underTest.Init(ctx))

		err := underTest.Start(ctx)
		synctest.Wait()

		assert.ErrorContains(t, err, "failed to ensure the existence of the cluster configuration: aborting test after some retries (")
		assert.ErrorIs(t, err, abortTest)
		assert.True(t, apierrors.IsNotFound(err))
	})
}

func TestClusterConfigInitializer_Exists(t *testing.T) {
	test := func(t *testing.T, leader bool) *testutil.FakeClientFactory {
		existingConfig := k0sv1beta1.DefaultClusterConfig()
		existingConfig.ResourceVersion = "42"
		clients := testutil.NewFakeClientFactory(existingConfig)
		leaderElector := leaderelector.Dummy{Leader: leader}
		initialConfig := existingConfig.DeepCopy()
		initialConfig.ResourceVersion = "1337"

		underTest := controller.NewClusterConfigInitializer(
			clients, &leaderElector, initialConfig,
		)

		require.NoError(t, underTest.Init(t.Context()))
		require.NoError(t, underTest.Start(t.Context()))
		t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

		synctest.Wait()

		actualConfig, err := clients.K0sClient.K0sV1beta1().
			ClusterConfigs(constant.ClusterConfigNamespace).
			Get(t.Context(), "k0s", metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.Equal(t, existingConfig, actualConfig)
		}

		return clients
	}

	t.Run("Leader", func(t *testing.T) { synctest.Test(t, func(t *testing.T) { test(t, true) }) })
	t.Run("Follower", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			clients := test(t, false)

			crds, err := clients.APIExtensionsClient.ApiextensionsV1().
				CustomResourceDefinitions().
				List(t.Context(), metav1.ListOptions{})
			if assert.NoError(t, err) {
				assert.Empty(t, crds.Items, "CRDs shouldn't be applied")
			}
		})
	})
}

func TestMain(m *testing.M) {
	internallog.SetDebugLevel()
	os.Exit(m.Run())
}
