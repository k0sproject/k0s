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
  name:  kube-system
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
      - name: nginx
        image: nginx:1.15
`

	templateDeployment := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
       app: nginx
    spec:
      containers:
      - name: nginx
        image: docker.io/nginx:1-alpine
        resources:
          limits:
            memory: "64Mi"
            cpu: "100m"
        ports:
          - containerPort: 80
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
	r, err := fakes.DynamicClient.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	podgv, _ := schema.ParseResourceArg("pods.v1.")
	r, err = fakes.DynamicClient.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Equal(t, "Pod", r.GetKind())
		assert.Equal(t, "applier", r.GetLabels()["component"])
	}
	deployGV, _ := schema.ParseResourceArg("deployments.v1.apps")
	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NoError(t, err)

	// Attempt to delete the stack with a different applier
	a2 := applier.NewApplier(dir, fakes)
	assert.NoError(t, a2.Delete(ctx))
	// Check that the resources are deleted
	_, err = fakes.DynamicClient.Resource(*gv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*podgv).Namespace("kube-system").Get(ctx, "applier-test", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	_, err = fakes.DynamicClient.Resource(*deployGV).Namespace("kube-system").Get(ctx, "nginx", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))

	gvNS, _ := schema.ParseResourceArg("namespaces.v1.")
	_, err = fakes.DynamicClient.Resource(*gvNS).Get(ctx, "kube-system", metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
}
