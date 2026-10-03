// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestLoadProfile(t *testing.T) {
	const prevProfile = `
name: fake
data:
  apiServerAddresses: |
    [127.10.10.1:9998, 127.10.10.2:9997]
  nodeLocalLoadBalancing: |
    {enabled: true}
  konnectivity: |
    {agentPort: 1337}
`
	workerConfigMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-config-fake-" + constant.KubernetesMajorMinorVersion,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			"nodeLocalLoadBalancing": "{enabled: false}",
			"konnectivity":           "{agentPort: 1337}",
		},
	}

	cacheDir := t.TempDir()

	prevProfilePath := filepath.Join(cacheDir, "worker-profile.yaml")
	require.NoError(t, os.WriteFile(prevProfilePath, []byte(prevProfile), 0644))

	var mu sync.Mutex
	var triedHosts []string

	clientFactory := func(host string) (kubernetes.Interface, error) {
		mu.Lock()
		triedHosts = append(triedHosts, host)
		numTried := len(triedHosts)
		defer mu.Unlock()

		// let only the last host return valid results
		var availableObjects []runtime.Object
		if numTried == 3 {
			availableObjects = []runtime.Object{&workerConfigMap}
		}

		return fake.NewSimpleClientset(availableObjects...), nil
	}

	workerProfile, err := func() (*Profile, error) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		timer := time.AfterFunc(10*time.Second, func() {
			assert.Fail(t, "Call to Loader.Load() took too long, check the logs for details")
			cancel()
		})
		defer timer.Stop()

		workerConfig, err := loadProfile(
			ctx,
			testLogger(t),
			clientFactory,
			cacheDir,
			"fake",
		)

		if t.Failed() {
			t.FailNow()
		}
		return workerConfig, err
	}()

	require.NoError(t, err)
	if assert.NotNil(t, workerProfile) {
		expected := v1beta1.DefaultNodeLocalLoadBalancing()
		assert.Equal(t, expected, workerProfile.NodeLocalLoadBalancing)
	}

	mu.Lock()
	theTriedHosts := triedHosts
	mu.Unlock()

	assert.ElementsMatch(t,
		[]string{"127.10.10.1:9998", "127.10.10.2:9997", ""},
		theTriedHosts,
		"Not all API server addresses were tried",
	)
}

func TestWatchProfile(t *testing.T) {
	workerConfigMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s-%s", constant.WorkerConfigComponentName, t.Name(), constant.KubernetesMajorMinorVersion),
			Namespace:       metav1.NamespaceSystem,
			ResourceVersion: t.Name(),
		},
		Data: map[string]string{
			"kubeletConfiguration": "{kind: foo}",
			"konnectivity":         "{agentPort: 1337}",
		},
	}

	client := fake.NewSimpleClientset(&workerConfigMap)
	client.PrependReactor("list", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.ConfigMapList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: t.Name(),
			},
			Items: []corev1.ConfigMap{workerConfigMap},
		}, nil
	})

	cacheDir := t.TempDir()

	ctx, cancel := context.WithCancelCause(t.Context())
	timeout := errors.New("timeout: Watcher.Watch()")
	timer := time.AfterFunc(10*time.Second, func() { cancel(timeout) })
	defer func() {
		timer.Stop()
		if errors.Is(context.Cause(ctx), timeout) {
			assert.Fail(t, "Call to Watcher.Watch() took too long, check the logs for details")
		}
	}()

	var timesCallbackCalled atomic.Uint32
	callback := func(p Profile) error {
		timesCallbackCalled.Add(1)
		assert.Equal(t, "foo", p.KubeletConfiguration.Kind)
		timer.Stop()
		cancel(nil)
		return nil
	}

	err := WatchProfile(ctx, testLogger(t), client, cacheDir, t.Name(), callback)
	assert.ErrorIs(t, err, ctx.Err())
	assert.Equal(t, uint32(1), timesCallbackCalled.Load())

	data, err := os.ReadFile(filepath.Join(cacheDir, "worker-profile.yaml"))
	require.NoError(t, err)
	var parsed struct{ Data map[string]string }
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	kubeConfigData, ok := parsed.Data["kubeletConfiguration"]
	require.True(t, ok)
	var kubeletConfig metav1.TypeMeta
	require.NoError(t, yaml.Unmarshal([]byte(kubeConfigData), &kubeletConfig))
	require.Equal(t, "foo", kubeletConfig.Kind)
}

func TestLoadProfileFromCache(t *testing.T) {
	t.Parallel()

	const cachedProfile = `
name: fake
kubernetesVersion: ` + constant.KubernetesMajorMinorVersion + `
data:
  nodeLocalLoadBalancing: |
    {enabled: true}
  konnectivity: |
    {agentPort: 1337}
`

	t.Run("returns_the_cached_profile", func(t *testing.T) {
		t.Parallel()

		profile, err := LoadProfileFromCache(cacheDirContaining(t, cachedProfile), "fake")

		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.True(t, profile.NodeLocalLoadBalancing.IsEnabled(), "Cached node-local load balancing settings weren't restored")
		assert.Equal(t, uint16(1337), profile.Konnectivity.AgentPort, "Cached Konnectivity settings weren't restored")
	})

	t.Run("reports_a_missing_cache_as_fs_ErrNotExist", func(t *testing.T) {
		t.Parallel()

		profile, err := LoadProfileFromCache(t.TempDir(), "fake")

		assert.ErrorIs(t, err, fs.ErrNotExist, "Callers should be able to detect a missing cache file")
		assert.Nil(t, profile)
	})

	for _, test := range []struct {
		name, content, profileName, errorContains string
	}{
		{"rejects_corrupt_caches", "name: [unterminated", "fake", ""},
		{"rejects_caches_for_another_profile", cachedProfile, "other", `cached worker profile is for profile "fake", not "other"`},
		{"rejects_caches_for_another_kubernetes_version", "name: fake\nkubernetesVersion: \"1.0\"\ndata: {}\n", "fake", "cached worker profile is for Kubernetes 1.0, not " + constant.KubernetesMajorMinorVersion},
		{"rejects_caches_without_a_kubernetes_version", "name: fake\ndata: {}\n", "fake", "cached worker profile doesn't record a Kubernetes version"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			profile, err := LoadProfileFromCache(cacheDirContaining(t, test.content), test.profileName)

			require.Error(t, err)
			if test.errorContains != "" {
				assert.ErrorContains(t, err, test.errorContains)
			}
			assert.Nil(t, profile)
		})
	}
}

func TestLoadProfile_Cache(t *testing.T) {
	t.Parallel()

	const cachedProfile = `
name: fake
kubernetesVersion: ` + constant.KubernetesMajorMinorVersion + `
data:
  konnectivity: |
    {agentPort: 1}
`

	t.Run("is_overwritten_when_the_load_succeeds", func(t *testing.T) {
		t.Parallel()

		cacheDir := cacheDirContaining(t, cachedProfile)
		clientFactory := func(string) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "worker-config-fake-" + constant.KubernetesMajorMinorVersion,
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string]string{"konnectivity": "{agentPort: 1337}"},
			}), nil
		}

		profile, err := loadProfile(t.Context(), testLogger(t), clientFactory, cacheDir, "fake")
		require.NoError(t, err)
		assert.Equal(t, uint16(1337), profile.Konnectivity.AgentPort)

		cached, err := LoadProfileFromCache(cacheDir, "fake")
		require.NoError(t, err)
		assert.Equal(t, uint16(1337), cached.Konnectivity.AgentPort, "The stale cache should have been replaced by the profile from the API")
	})

	t.Run("is_left_untouched_when_the_load_is_aborted", func(t *testing.T) {
		t.Parallel()

		cacheDir := cacheDirContaining(t, cachedProfile)
		clientFactory := func(string) (kubernetes.Interface, error) {
			return nil, assert.AnError
		}

		ctx, cancel := context.WithCancel(t.Context())
		cancel() // don't wait for the retries to be exhausted
		profile, err := loadProfile(ctx, testLogger(t), clientFactory, cacheDir, "fake")
		require.Error(t, err)
		assert.Nil(t, profile)

		cached, err := LoadProfileFromCache(cacheDir, "fake")
		require.NoError(t, err)
		assert.Equal(t, uint16(1), cached.Konnectivity.AgentPort, "A failed load shouldn't modify the cached worker profile")
	})
}

func TestLoadProfile_APIReachability(t *testing.T) {
	t.Parallel()

	// The errors are marked unrecoverable so that the tests don't have to sit
	// through the retry backoff. retry-go unpacks them again before returning.
	clientFactoryFailingWith := func(err error) func(string) (kubernetes.Interface, error) {
		return func(string) (kubernetes.Interface, error) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, retry.Unrecoverable(err)
			})
			return client, nil
		}
	}

	t.Run("rejections_by_the_api_server_are_not_unreachable", func(t *testing.T) {
		t.Parallel()

		clientFactory := clientFactoryFailingWith(apierrors.NewUnauthorized("no credentials provided"))
		_, err := loadProfile(t.Context(), testLogger(t), clientFactory, t.TempDir(), "fake")

		require.Error(t, err)
		assert.False(t, IsAPIUnreachable(err), "The API server responded, so it was reachable")
		assert.True(t, apierrors.IsUnauthorized(err), "The rejection should remain detectable")
	})

	t.Run("transport_failures_are_unreachable", func(t *testing.T) {
		t.Parallel()

		clientFactory := clientFactoryFailingWith(&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")})
		_, err := loadProfile(t.Context(), testLogger(t), clientFactory, t.TempDir(), "fake")

		require.Error(t, err)
		assert.True(t, IsAPIUnreachable(err), "The API server never responded, so it was unreachable")
	})
}

func cacheDirContaining(t *testing.T, content string) string {
	t.Helper()

	cacheDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "worker-profile.yaml"), []byte(content), 0644))
	return cacheDir
}

func testLogger(t *testing.T) logrus.FieldLogger {
	t.Helper()

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return log.WithField("test", t.Name())
}
