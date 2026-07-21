// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	k0sfake "github.com/k0sproject/k0s/pkg/client/clientset/fake"
	k0sclient "github.com/k0sproject/k0s/pkg/client/clientset/typed/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: Remove in k0s 1.38+: Sanitize feature gates.
func TestAPIConfigSource_SanitizesFeatureGates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stored := insaneClusterConfig()
		expected := sanitizedFeatureGates()

		client := k0sfake.NewSimpleClientset(stored).K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)
		underTest := startAPIConfigSource(t, client)

		// The internally published config is sanitized.
		published := <-underTest.ResultChan()
		require.NotNil(t, published.Spec)
		assert.Equal(t, expected, published.Spec.FeatureGates)

		// Wait until the leader task has written the sanitized config back.
		synctest.Wait()

		written, err := client.Get(t.Context(), constant.ClusterConfigObjectName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, written.Spec)
		assert.Equal(t, expected, written.Spec.FeatureGates,
			"Sanitized config should have been written back to the API")
	})
}

// TODO: Remove in k0s 1.38+: Sanitize feature gates.
func TestAPIConfigSource_RetriesFailedWriteBacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stored := insaneClusterConfig()
		expected := sanitizedFeatureGates()

		clients := k0sfake.NewSimpleClientset(stored)
		var updates atomic.Uint32
		clients.PrependReactor("update", "clusterconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
			if updates.Add(1) == 1 {
				return true, nil, apierrors.NewInternalError(assert.AnError)
			}
			return false, nil, nil
		})
		client := clients.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)
		underTest := startAPIConfigSource(t, client)

		published := <-underTest.ResultChan()
		require.NotNil(t, published.Spec)
		assert.Equal(t, expected, published.Spec.FeatureGates)

		// The first write-back attempt fails and leaves the API untouched.
		synctest.Wait()
		assert.EqualValues(t, 1, updates.Load())
		written, err := client.Get(t.Context(), constant.ClusterConfigObjectName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, written.Spec)
		assert.Len(t, written.Spec.FeatureGates, 3, "Nothing should have been written back yet")

		// The retry fires after at most 70 seconds (50 seconds plus jitter).
		time.Sleep(70 * time.Second)
		synctest.Wait()

		assert.EqualValues(t, 2, updates.Load())
		written, err = client.Get(t.Context(), constant.ClusterConfigObjectName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, written.Spec)
		assert.Equal(t, expected, written.Spec.FeatureGates,
			"Sanitized config should have been written back on the second attempt")
	})
}

// TODO: Remove in k0s 1.38+: Sanitize feature gates.
func TestAPIConfigSource_ResolvesConflictsViaWatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		stored := insaneClusterConfig()
		expected := sanitizedFeatureGates()

		clients := k0sfake.NewSimpleClientset(stored)
		var updates atomic.Uint32
		clients.PrependReactor("update", "clusterconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
			if updates.Add(1) == 1 {
				return true, nil, apierrors.NewConflict(
					schema.GroupResource{Group: v1beta1.GroupName, Resource: "clusterconfigs"},
					constant.ClusterConfigObjectName, assert.AnError,
				)
			}
			return false, nil, nil
		})
		client := clients.K0sV1beta1().ClusterConfigs(constant.ClusterConfigNamespace)
		underTest := startAPIConfigSource(t, client)

		published := <-underTest.ResultChan()
		require.NotNil(t, published.Spec)
		assert.Equal(t, expected, published.Spec.FeatureGates)

		// The write-back hits a conflict. No retry timer may be armed for
		// conflicts: the concurrent change will be seen by the watch.
		synctest.Wait()
		time.Sleep(24 * time.Hour)
		synctest.Wait()
		assert.EqualValues(t, 1, updates.Load(), "Conflicts shouldn't be retried on a timer")

		// The concurrent change arrives via the watch, still insane.
		concurrent := insaneClusterConfig()
		concurrent.ResourceVersion = "2"
		_, err := client.Update(t.Context(), concurrent, metav1.UpdateOptions{})
		require.NoError(t, err)

		published = <-underTest.ResultChan()
		require.NotNil(t, published.Spec)
		assert.Equal(t, expected, published.Spec.FeatureGates)

		// Now the write-back succeeds against the new version.
		synctest.Wait()
		assert.EqualValues(t, 3, updates.Load())
		written, err := client.Get(t.Context(), constant.ClusterConfigObjectName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, written.Spec)
		assert.Equal(t, expected, written.Spec.FeatureGates,
			"Sanitized config should have been written back after the conflict")
	})
}

// TODO: Remove in k0s 1.38+: Sanitize feature gates.
func insaneClusterConfig() *v1beta1.ClusterConfig {
	return &v1beta1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            constant.ClusterConfigObjectName,
			Namespace:       constant.ClusterConfigNamespace,
			ResourceVersion: "1",
		},
		Spec: &v1beta1.ClusterSpec{
			FeatureGates: v1beta1.FeatureGates{
				{Name: "KeepMe", Enabled: true},
				{Name: "StripMe", Enabled: true, Components: []v1beta1.FeatureComponent{
					"bogus", v1beta1.FeatureComponentKubelet,
				}},
				{Name: "DropMe", Components: []v1beta1.FeatureComponent{"bogus"}},
			},
		},
	}
}

// TODO: Remove in k0s 1.38+: Sanitize feature gates.
func sanitizedFeatureGates() v1beta1.FeatureGates {
	return v1beta1.FeatureGates{
		{Name: "KeepMe", Enabled: true},
		{Name: "StripMe", Enabled: true, Components: []v1beta1.FeatureComponent{
			v1beta1.FeatureComponentKubelet,
		}},
	}
}

func startAPIConfigSource(t *testing.T, client k0sclient.ClusterConfigInterface) *apiConfigSource {
	underTest := &apiConfigSource{
		configClient: client,
		leaderStatus: func() (leaderelection.Status, <-chan struct{}) {
			return leaderelection.StatusLeading, nil // the lead never expires
		},
		resultChan: make(chan *v1beta1.ClusterConfig),
	}

	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

	return underTest
}
