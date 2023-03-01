/*
Copyright 2022 k0s authors

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

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

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
			Name:      fmt.Sprintf("worker-config-fake-%s", constant.KubernetesMajorMinorVersion),
			Namespace: "kube-system",
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
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()
		timer := time.AfterFunc(10*time.Second, func() {
			assert.Fail(t, "Call to Loader.Load() took too long, check the logs for details")
			cancel()
		})
		defer timer.Stop()

		log := logrus.New()
		log.SetLevel(logrus.DebugLevel)

		workerConfig, err := loadProfile(
			ctx,
			log.WithField("test", t.Name()),
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
			Namespace:       "kube-system",
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

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	timer := time.AfterFunc(10*time.Second, func() {
		assert.Fail(t, "Call to Watcher.Watch() took too long, check the logs for details")
		cancel()
	})
	defer timer.Stop()

	var timesCallbackCalled atomic.Uint32
	callback := func(p Profile) error {
		timesCallbackCalled.Add(1)
		assert.Equal(t, "foo", p.KubeletConfiguration.Kind)
		timer.Stop()
		cancel()
		return nil
	}

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	err := WatchProfile(ctx, log.WithField("test", t.Name()), client, cacheDir, t.Name(), callback)
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
