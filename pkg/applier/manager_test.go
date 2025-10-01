// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/k0sproject/k0s/internal/sync/value"
	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/k0sproject/k0s/pkg/leaderelection"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	yaml "sigs.k8s.io/yaml/goyaml.v2"

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

	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, leaderElector.Stop()) })

	// Wait for the "before" stack to be applied.
	require.NoError(t, watch.ConfigMaps(clients.Client.CoreV1().ConfigMaps("default")).
		Until(t.Context(), func(item *corev1.ConfigMap) (bool, error) {
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
		Until(t.Context(), func(item *corev1.ConfigMap) (bool, error) {
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

	require.NoError(t, leaderElector.Init(t.Context()))
	require.NoError(t, underTest.Init(t.Context()))
	require.NoError(t, underTest.Start(t.Context()))
	t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })
	require.NoError(t, leaderElector.Start(t.Context()))
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

	configMaps, err := clients.Client.CoreV1().ConfigMaps("default").List(t.Context(), metav1.ListOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, configMaps.Items)
	}
}

//go:embed testdata/manager_test/*
var managerTestData embed.FS

func TestManager(t *testing.T) {
	ctx := t.Context()

	dir := t.TempDir()

	cfg := &config.CfgVars{
		ManifestsDir: dir,
	}

	fakes := testutil.NewFakeClientFactory()

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
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		r, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		if assert.NoError(t, err) {
			assert.Equal(t, "test", r.GetLabels()["custom1"])
		}
	}, 5*time.Second, 100*time.Millisecond)

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

	// update the stack after the lease acquire and verify the changes are applied

	writeLabel(t, filepath.Join(dir, "stack1/pod.yaml"), "custom2", "test")

	t.Log("waiting for pod to be updated")
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		r, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		if assert.NoError(t, err) {
			assert.Equal(t, "test", r.GetLabels()["custom2"])
		}
	}, 5*time.Second, 100*time.Millisecond)

	// delete the stack and verify the resources are deleted

	err = os.RemoveAll(filepath.Join(dir, "stack1"))
	require.NoError(t, err)

	t.Log("waiting for pod to be deleted")
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := getResource(fakes, *podgv, "kube-system", "applier-test")
		assert.Truef(t, errors.IsNotFound(err), "Expected a 'not found' error: %v", err)
	}, 5*time.Second, 100*time.Millisecond)
}

func writeLabel(t *testing.T, file string, key string, value string) {
	t.Helper()
	contents, err := os.ReadFile(file)
	require.NoError(t, err)
	unst := map[any]any{}
	err = yaml.Unmarshal(contents, &unst)
	require.NoError(t, err)
	unst["metadata"].(map[any]any)["labels"].(map[any]any)[key] = value
	data, err := yaml.Marshal(unst)
	require.NoError(t, err)
	err = os.WriteFile(file, data, 0400)
	require.NoError(t, err)
}

func waitForResource(t *testing.T, fakes *testutil.FakeClientFactory, gv schema.GroupVersionResource, namespace string, name string) {
	t.Logf("waiting for resource %s/%s", gv.Resource, name)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := getResource(fakes, gv, namespace, name)
		if err != nil {
			require.Truef(t, errors.IsNotFound(err), "Expected a 'not found' error: %v", err)
			assert.NoError(c, err)
		}
	}, 5*time.Second, 100*time.Millisecond)
}

func getResource(fakes *testutil.FakeClientFactory, gv schema.GroupVersionResource, namespace string, name string) (*unstructured.Unstructured, error) {
	return fakes.DynamicClient.Resource(gv).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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
	leader   value.Latest[bool]
	acquired []func()
	lost     []func()
}

func (e *mockLeaderElector) activate() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if leader, _ := e.leader.Peek(); !leader {
		e.leader.Set(true)
		for _, fn := range e.acquired {
			fn()
		}
	}
}

func (e *mockLeaderElector) deactivate() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if leader, _ := e.leader.Peek(); leader {
		e.leader.Set(false)
		for _, fn := range e.lost {
			fn()
		}
	}
}

func (e *mockLeaderElector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	leader, _ := e.leader.Peek()
	return leader
}

func (e *mockLeaderElector) CurrentStatus() (status leaderelection.Status, expired <-chan struct{}) {
	leader, expired := e.leader.Peek()
	if leader {
		return leaderelection.StatusLeading, expired
	}
	return leaderelection.StatusPending, expired
}

func (e *mockLeaderElector) AddAcquiredLeaseCallback(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acquired = append(e.acquired, fn)
	if leader, _ := e.leader.Peek(); leader {
		fn()
	}
}

func (e *mockLeaderElector) AddLostLeaseCallback(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lost = append(e.lost, fn)
	if leader, _ := e.leader.Peek(); !leader {
		fn()
	}
}
