// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubeutil "github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/applier"
)

func TestApplierAppliesAllManifestsInADirectory(t *testing.T) {
	dir := t.TempDir()
	templateNS := `
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
`
	template := `
apiVersion: v1
kind: List
items:
  - kind: ConfigMap
    apiVersion: v1
    metadata:
      name: applier-test
      namespace: kube-system
      labels:
        component: applier
    data:
      foo: bar
  - kind: Pod
    apiVersion: v1
    metadata:
      name: applier-test
      namespace: kube-system
      labels:
        component: applier
    spec:
      containers:
      - name: app
        image: registry.example.com/some/app:1
`

	templateDeployment := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: app
  template:
    metadata:
      labels:
       app: app
    spec:
      containers:
      - name: app
        image: registry.example.com/some/app:1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-ns.yaml"), []byte(templateNS), 0400))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-list.yaml"), []byte(template), 0400))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test-deploy.yaml"), []byte(templateDeployment), 0400))

	fakes := kubeutil.NewFakeClientFactory()
	a := applier.NewApplier(dir, fakes)

	ctx := t.Context()
	err := a.Apply(ctx)
	assert.NoError(t, err)
	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	r, err := fakes.DynamicClient.Resource(*gv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = fakes.DynamicClient.Resource(*podgv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "Pod", r.GetKind())
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	deployGV, _ := schema.ParseResourceArg("deployments.v1.apps")
	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace(metav1.NamespaceSystem).Get(ctx, "app", metav1.GetOptions{})
	assert.NoError(t, err)

	// Attempt to delete the stack with a different applier
	a2 := applier.NewApplier(dir, fakes)
	assert.NoError(t, a2.Delete(ctx))
	// Check that the resources are deleted
	_, err = fakes.DynamicClient.Resource(*gv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*podgv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace(metav1.NamespaceSystem).Get(ctx, "app", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	gvNS, _ := schema.ParseResourceArg("namespaces.v1.")
	_, err = fakes.DynamicClient.Resource(*gvNS).Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
}

// TestApplierApplyDoesNotPruneOnEmptyDirectory verifies that Apply does not
// prune previously applied resources when the stack directory is (transiently)
// empty of manifest files, e.g. because the owning component hasn't (re-)written
// its manifest yet. Only an explicit Delete, triggered by removal of the whole
// stack directory, is supposed to remove a stack's resources.
// https://github.com/k0sproject/k0s/issues/8214
func TestApplierApplyDoesNotPruneOnEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	templateNS := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: applier-test
  namespace: kube-system
  labels:
    component: applier
data:
  foo: bar
`
	manifestPath := filepath.Join(dir, "test-cm.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(templateNS), 0400))

	fakes := kubeutil.NewFakeClientFactory()
	a := applier.NewApplier(dir, fakes)

	ctx := t.Context()
	require.NoError(t, a.Apply(ctx))

	gv, _ := schema.ParseResourceArg("configmaps.v1.")
	_, err := fakes.DynamicClient.Resource(*gv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	require.NoError(t, err)

	// Simulate a restart of the owning component before it has (re-)written
	// its manifest: the manifest file is gone, but the stack directory is not.
	require.NoError(t, os.Remove(manifestPath))

	require.NoError(t, a.Apply(ctx))

	// The resource must still be there: an empty directory must not be
	// treated as an implicit request to delete the whole stack.
	_, err = fakes.DynamicClient.Resource(*gv).Namespace(metav1.NamespaceSystem).Get(ctx, "applier-test", metav1.GetOptions{})
	assert.NoError(t, err)
}
