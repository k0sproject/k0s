/*
Copyright 2024 k0s authors

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

package applier_test

import (
	"context"
	"embed"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	yaml "sigs.k8s.io/yaml/goyaml.v2"

	kubeutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_AppliesStacks(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()

	underTest := applier.Manager{
		K0sVars:           k0sVars,
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}

	// A stack that exists on disk before the manager is started.
	before := filepath.Join(k0sVars.ManifestsDir, "before")
	require.NoError(t, os.MkdirAll(before, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(before, "before.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: before
  namespace: default
  resourceVersion: "1"
data: {}
`,
	), constant.CertMode))

	require.NoError(t, leaderElector.Init(context.TODO()))
	require.NoError(t, underTest.Init(context.TODO()))
	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })

	// Wait for the "before" stack to be applied.
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps("default")).
		Until(context.TODO(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "before", nil
		}),
	)

	// A stack that is created on disk after the manager has started.
	after := filepath.Join(k0sVars.ManifestsDir, "after")
	require.NoError(t, os.MkdirAll(after, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(after, "after.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: after
  namespace: default
  resourceVersion: "1"
data: {}
`,
	), constant.CertMode))

	// Wait for the "after" stack to be applied.
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps("default")).
		Until(context.TODO(), func(item *corev1.ConfigMap) (bool, error) {
			return item.Name == "after", nil
		}),
	)
}

func TestManager_IgnoresStacks(t *testing.T) {
	k0sVars, err := config.NewCfgVars(nil, t.TempDir())
	require.NoError(t, err)
	leaderElector := leaderelector.Dummy{Leader: true}
	clients := testutil.NewFakeClientFactory()

	underTest := applier.Manager{
		K0sVars:           k0sVars,
		IgnoredStacks:     []string{"ignored"},
		KubeClientFactory: clients,
		LeaderElector:     &leaderElector,
	}

	ignored := filepath.Join(k0sVars.ManifestsDir, "ignored")
	require.NoError(t, os.MkdirAll(ignored, constant.ManifestsDirMode))
	require.NoError(t, os.WriteFile(filepath.Join(ignored, "ignored.yaml"), []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignored
  namespace: default
data: {}
`,
	), constant.CertMode))

	require.NoError(t, leaderElector.Init(context.TODO()))
	require.NoError(t, underTest.Init(context.TODO()))
	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(context.TODO()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })

	var content []byte
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		content, err = os.ReadFile(filepath.Join(ignored, "ignored.txt"))
		assert.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	expectedContent := []string{
		"The ignored stack is handled internally.",
		"This directory is ignored and can be safely removed.",
		"",
	}
	assert.Equal(t, strings.Join(expectedContent, "\n"), string(content))

	configMaps, err := clients.Client.CoreV1().ConfigMaps("default").List(context.TODO(), metav1.ListOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, configMaps.Items)
	}
}

//go:embed testdata/manager_test/*
var managerTestData embed.FS

func TestManager(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()

	cfg := &config.CfgVars{
		ManifestsDir: dir,
	}

	fakes := kubeutil.NewFakeClientFactory()

	le := new(mockLeaderElector)

	manager := &applier.Manager{
		K0sVars:           cfg,
		KubeClientFactory: fakes,
		LeaderElector:     le,
	}

	writeStack(t, dir, "testdata/manager_test/stack1")

	err := manager.Init(ctx)
	require.NoError(t, err)

	err = manager.Start(ctx)
	require.NoError(t, err)

	le.activate()

	// validate stack that already exists is applied

	cmgv, _ := schema.ParseResourceArg("configmaps.v1.")
	podgv, _ := schema.ParseResourceArg("pods.v1.")

	waitForResource(t, fakes, *cmgv, "kube-system", "applier-test")
	waitForResource(t, fakes, *podgv, "kube-system", "applier-test")

	r, err := getResource(fakes, *cmgv, "kube-system", "applier-test")
	if assert.NoError(t, err) {
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	r, err = getResource(fakes, *podgv, "kube-system", "applier-test")
	if assert.NoError(t, err) {
		assert.Equal(t, "Pod", r.GetKind())
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}

	// update the stack and verify the changes are applied

	writeLabel(t, filepath.Join(dir, "stack1/pod.yaml"), "custom1", "test")

	t.Log("waiting for pod to be updated")
	waitFor(t, 100*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		r, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		if err != nil {
			return false, nil
		}
		return r.GetLabels()["custom1"] == "test", nil
	})

	// lose and re-acquire leadership
	le.deactivate()
	le.activate()

	// validate a new stack that is added is applied

	writeStack(t, dir, "testdata/manager_test/stack2")

	deployGV, _ := schema.ParseResourceArg("deployments.v1.apps")

	waitForResource(t, fakes, *deployGV, "kube-system", "nginx")

	r, err = getResource(fakes, *deployGV, "kube-system", "nginx")
	if assert.NoError(t, err) {
		assert.Equal(t, "Deployment", r.GetKind())
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}

	// update the stack after the lease aquire and verify the changes are applied

	writeLabel(t, filepath.Join(dir, "stack1/pod.yaml"), "custom2", "test")

	t.Log("waiting for pod to be updated")
	waitFor(t, 100*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		r, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		if err != nil {
			return false, nil
		}
		return r.GetLabels()["custom2"] == "test", nil
	})

	// delete the stack and verify the resources are deleted

	err = os.RemoveAll(filepath.Join(dir, "stack1"))
	require.NoError(t, err)

	t.Log("waiting for pod to be deleted")
	waitFor(t, 100*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		_, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

func writeLabel(t *testing.T, file string, key string, value string) {
	t.Helper()
	contents, err := os.ReadFile(file)
	require.NoError(t, err)
	unst := map[interface{}]interface{}{}
	err = yaml.Unmarshal(contents, &unst)
	require.NoError(t, err)
	unst["metadata"].(map[interface{}]interface{})["labels"].(map[interface{}]interface{})[key] = value
	data, err := yaml.Marshal(unst)
	require.NoError(t, err)
	err = os.WriteFile(file, data, 0400)
	require.NoError(t, err)
}

func waitForResource(t *testing.T, fakes *kubeutil.FakeClientFactory, gv schema.GroupVersionResource, namespace string, name string) {
	t.Logf("waiting for resource %s/%s", gv.Resource, name)
	waitFor(t, 100*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		_, err := getResource(fakes, gv, namespace, name)
		if errors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
}

func getResource(fakes *kubeutil.FakeClientFactory, gv schema.GroupVersionResource, namespace string, name string) (*unstructured.Unstructured, error) {
	return fakes.DynamicClient.Resource(gv).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func waitFor(t *testing.T, interval, timeout time.Duration, fn wait.ConditionWithContextFunc) {
	t.Helper()
	err := wait.PollUntilContextTimeout(context.Background(), interval, timeout, true, fn)
	require.NoError(t, err)
}

func writeStack(t *testing.T, dst string, src string) {
	dstStackDir := filepath.Join(dst, path.Base(src))
	err := os.MkdirAll(dstStackDir, 0755)
	require.NoError(t, err)
	entries, err := managerTestData.ReadDir(src)
	require.NoError(t, err)
	for _, entry := range entries {
		data, err := managerTestData.ReadFile(path.Join(src, entry.Name()))
		require.NoError(t, err)
		dst := filepath.Join(dstStackDir, entry.Name())
		t.Logf("writing file %s", dst)
		err = os.WriteFile(dst, data, 0644)
		require.NoError(t, err)
	}
}

type mockLeaderElector struct {
	mu       sync.Mutex
	leader   bool
	acquired []func()
	lost     []func()
}

func (e *mockLeaderElector) activate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.leader {
		e.leader = true
		for _, fn := range e.acquired {
			fn()
		}
	}
}

func (e *mockLeaderElector) deactivate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.leader {
		e.leader = false
		for _, fn := range e.lost {
			fn()
		}
	}
}

func (e *mockLeaderElector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.leader
}

func (e *mockLeaderElector) AddAcquiredLeaseCallback(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acquired = append(e.acquired, fn)
	if e.leader {
		fn()
	}
}

func (e *mockLeaderElector) AddLostLeaseCallback(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lost = append(e.lost, fn)
	if e.leader {
		fn()
	}
}
